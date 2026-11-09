package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testImage(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 120, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 120; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
}
func setup(t *testing.T) (*Store, http.Handler) {
	t.Helper()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s, NewServer(s, DemoProvider{}, "", t.TempDir()).Handler()
}
func call(t *testing.T, h http.Handler, method, path string, input any) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	if input != nil {
		if err := json.NewEncoder(&b).Encode(input); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, "http://localhost"+path, &b)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func enroll(t *testing.T, h http.Handler) SubjectView {
	t.Helper()
	w := call(t, h, "POST", "/api/subjects", map[string]any{"name": "Demo A", "image": testImage(t), "consent": true})
	if w.Code != 201 {
		t.Fatalf("enroll %d %s", w.Code, w.Body)
	}
	var v SubjectView
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestHTTPFullFlowAndDeletion(t *testing.T) {
	s, h := setup(t)
	subject := enroll(t, h)
	w := call(t, h, "GET", "/api/subjects", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "photo") {
		t.Fatal(w.Body)
	}
	w = call(t, h, "POST", "/api/verifications", map[string]any{"subject_id": subject.ID, "image": testImage(t), "consent": true})
	var v Record
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if w.Code != 201 || v.Decision != "demo" || v.Score != nil || v.Mode != "demo" {
		t.Fatalf("invalid demo result: %d %+v", w.Code, v)
	}
	if len(s.Records()) != 1 {
		t.Fatal("missing record")
	}
	w = call(t, h, "DELETE", "/api/subjects/"+subject.ID, nil)
	if w.Code != 204 || len(s.Records()) != 0 || len(s.Subjects()) != 0 {
		t.Fatal("cascade deletion failed")
	}
	w = call(t, h, "POST", "/api/verifications", map[string]any{"subject_id": subject.ID, "image": testImage(t), "consent": true})
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
func TestConsentImageAndInputValidation(t *testing.T) {
	_, h := setup(t)
	for _, input := range []map[string]any{
		{"name": "demo", "image": testImage(t), "consent": false},
		{"name": " ", "image": testImage(t), "consent": true},
		{"name": "demo", "image": "not an image", "consent": true},
		{"name": "demo", "image": testImage(t), "consent": true, "extra": 1},
	} {
		if w := call(t, h, "POST", "/api/subjects", input); w.Code != 400 {
			t.Fatalf("expected 400 got %d", w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://localhost/api/subjects", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("POST", "http://localhost/api/subjects", strings.NewReader(`{"image":"`+strings.Repeat("A", 6<<20)+`"}`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatal(w.Code)
	}
}
func TestSecurityAndConcurrencyLimit(t *testing.T) {
	s, _ := setup(t)
	server := NewServer(s, DemoProvider{}, "demo-token", t.TempDir())
	h := server.Handler()
	if w := call(t, h, "GET", "/api/subjects", nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("GET", "http://localhost/api/subjects", nil)
	r.Header.Set("Authorization", "Bearer demo-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	r.Header.Set("Origin", "https://different.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("GET", "http://evil.example/api/subjects", nil)
	w = httptest.NewRecorder()
	NewServer(s, DemoProvider{}, "", "").Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	server.slots <- struct{}{}
	server.slots <- struct{}{}
	r = httptest.NewRequest("POST", "http://localhost/api/subjects", nil)
	r.Header.Set("Authorization", "Bearer demo-token")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 429 {
		t.Fatal(w.Code)
	}
	<-server.slots
	<-server.slots
}
func TestPersistenceAndFailedCommit(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.AddSubject("A", "photo")
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := OpenStore(dir)
	if err != nil || len(reloaded.Subjects()) != 1 {
		t.Fatalf("reload: %v", err)
	}
	s.path = filepath.Join(dir, "missing", "state.json")
	if _, err = s.AddSubject("B", "photo"); err == nil {
		t.Fatal("expected persistence error")
	}
	if len(s.Subjects()) != 1 {
		t.Fatal("failed commit changed memory")
	}
	if _, err = reloaded.AddRecord(Record{SubjectID: p.ID, Mode: "demo", Decision: "demo"}); err != nil {
		t.Fatal(err)
	}
	if err = reloaded.DeleteSubject(p.ID); err != nil {
		t.Fatal(err)
	}
	final, err := OpenStore(dir)
	if err != nil || len(final.Subjects()) != 0 || len(final.Records()) != 0 {
		t.Fatalf("deletion not durable: %v", err)
	}
	if err = os.WriteFile(filepath.Join(dir, "state.json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenStore(dir); err == nil {
		t.Fatal("corrupt state silently reset")
	}
}
func TestStoreConcurrentAccessAndDeletionRace(t *testing.T) {
	s, _ := setup(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := s.AddSubject(fmt.Sprint(i), "photo"); err != nil {
				t.Error(err)
			}
			_ = s.Subjects()
			_ = s.Records()
		}(i)
	}
	wg.Wait()
	if len(s.Subjects()) != 12 {
		t.Fatal("lost updates")
	}
	p := s.Subjects()[0]
	if err := s.DeleteSubject(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRecord(Record{SubjectID: p.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatal("record recreated after deletion")
	}
}
func TestImageNormalization(t *testing.T) {
	image, err := normalizeImage(testImage(t))
	if err != nil || strings.HasPrefix(image, "data:") {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(image)
	if err != nil || http.DetectContentType(raw) != "image/jpeg" {
		t.Fatal("not normalized JPEG")
	}
	for _, data := range []string{"data:image/svg+xml;base64,PHN2Zz4=", base64.StdEncoding.EncodeToString([]byte("not image")), strings.Repeat("A", 5<<20)} {
		if _, err := normalizeImage(data); err == nil {
			t.Fatal("invalid image accepted")
		}
	}
}
func TestRemoteProviderDecisions(t *testing.T) {
	for _, tc := range []struct {
		name, body, decision string
		fail                 bool
	}{
		{"match", `{"matched":true,"score":0.95,"face_count":1,"liveness":"passed"}`, "matched", false},
		{"threshold", `{"matched":true,"score":0.7,"face_count":1,"liveness":"passed"}`, "not_matched", false},
		{"no-live", `{"matched":true,"score":0.95,"face_count":1,"liveness":"unknown"}`, "review", false},
		{"multi-face", `{"matched":true,"score":0.95,"face_count":2,"liveness":"passed"}`, "review", false},
		{"missing", `{"score":0.95}`, "", true},
		{"invalid", `{"matched":true,"score":3,"face_count":1,"liveness":"passed"}`, "", true},
		{"trailing", `{"matched":true,"score":0.95,"face_count":1,"liveness":"passed"}extra`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("missing provider token")
				}
				var input map[string]string
				if json.NewDecoder(r.Body).Decode(&input) != nil || input["reference_image"] != "ref" || input["probe_image"] != "probe" {
					t.Error("invalid image request")
				}
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			p, err := NewRemoteProvider(server.URL, "test-token", 0.8)
			if err != nil {
				t.Fatal(err)
			}
			v, err := p.Verify(context.Background(), "ref", "probe")
			if (err != nil) != tc.fail || v.Decision != tc.decision {
				t.Fatalf("%+v %v", v, err)
			}
		})
	}
}

type failedProvider struct{}

func (failedProvider) Mode() string { return "remote" }
func (failedProvider) Verify(context.Context, string, string) (Result, error) {
	return Result{}, errors.New("secret upstream failure")
}
func TestProviderFailureDoesNotCreateRecord(t *testing.T) {
	s, _ := setup(t)
	h := NewServer(s, failedProvider{}, "", t.TempDir()).Handler()
	p := enroll(t, h)
	w := call(t, h, "POST", "/api/verifications", map[string]any{"subject_id": p.ID, "image": testImage(t), "consent": true})
	if w.Code != 502 || len(s.Records()) != 0 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Body)
	}
}
func TestRecordRetention(t *testing.T) {
	s, _ := setup(t)
	p, err := s.AddSubject("A", "photo")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 203; i++ {
		if _, err := s.AddRecord(Record{SubjectID: p.ID, Message: fmt.Sprint(i)}); err != nil {
			t.Fatal(err)
		}
	}
	rows := s.Records()
	if len(rows) != 200 || rows[0].Message != "202" || rows[199].Message != "3" {
		t.Fatal("retention wrong")
	}
}
