package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrNotFound = errors.New("人员不存在或已删除")
var ErrCapacity = errors.New("演示存储容量已满，请先删除不再需要的人员")

type Subject struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Photo     string    `json:"photo"`
}
type SubjectView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
type Record struct {
	ID        string    `json:"id"`
	SubjectID string    `json:"subject_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Mode      string    `json:"mode"`
	Decision  string    `json:"decision"`
	Score     *float64  `json:"score,omitempty"`
	Message   string    `json:"message"`
}
type state struct {
	Version  int       `json:"version"`
	Subjects []Subject `json:"subjects"`
	Records  []Record  `json:"records"`
}
type Store struct {
	mu   sync.RWMutex
	path string
	data state
}

func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "state.json"), data: state{Version: 1, Subjects: []Subject{}, Records: []Record{}}}
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err = dec.Decode(&s.data); err != nil {
		return nil, fmt.Errorf("存储损坏，拒绝覆盖: %w", err)
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("存储尾部数据无效")
	}
	if s.data.Version != 1 || s.data.Subjects == nil || s.data.Records == nil {
		return nil, errors.New("不支持的存储格式")
	}
	return s, nil
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func (s *Store) commit(next state) error {
	f, err := os.CreateTemp(filepath.Dir(s.path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = json.NewEncoder(f).Encode(next); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), s.path); err != nil {
		return err
	}
	s.data = next
	return nil
}
func (s *Store) snapshot() state {
	return state{Version: 1, Subjects: append([]Subject{}, s.data.Subjects...), Records: append([]Record{}, s.data.Records...)}
}
func (s *Store) Subjects() []SubjectView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SubjectView, 0, len(s.data.Subjects))
	for _, v := range s.data.Subjects {
		out = append(out, SubjectView{v.ID, v.Name, v.CreatedAt})
	}
	return out
}
func (s *Store) Subject(id string) (Subject, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.data.Subjects {
		if v.ID == id {
			return v, nil
		}
	}
	return Subject{}, ErrNotFound
}
func (s *Store) AddSubject(name, photo string) (SubjectView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Subjects) >= 50 {
		return SubjectView{}, ErrCapacity
	}
	id, err := newID()
	if err != nil {
		return SubjectView{}, err
	}
	v := Subject{ID: id, Name: name, Photo: photo, CreatedAt: time.Now().UTC()}
	next := s.snapshot()
	next.Subjects = append(next.Subjects, v)
	if err = s.commit(next); err != nil {
		return SubjectView{}, err
	}
	return SubjectView{v.ID, v.Name, v.CreatedAt}, nil
}
func (s *Store) DeleteSubject(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.snapshot()
	index := -1
	for i, v := range next.Subjects {
		if v.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return ErrNotFound
	}
	next.Subjects = append(next.Subjects[:index], next.Subjects[index+1:]...)
	kept := make([]Record, 0, len(next.Records))
	for _, v := range next.Records {
		if v.SubjectID != id {
			kept = append(kept, v)
		}
	}
	next.Records = kept
	return s.commit(next)
}
func (s *Store) AddRecord(v Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for _, subject := range s.data.Subjects {
		if subject.ID == v.SubjectID {
			found = true
			v.Name = subject.Name
			break
		}
	}
	if !found {
		return Record{}, ErrNotFound
	}
	id, err := newID()
	if err != nil {
		return Record{}, err
	}
	v.ID = id
	v.CreatedAt = time.Now().UTC()
	next := s.snapshot()
	next.Records = append([]Record{v}, next.Records...)
	if len(next.Records) > 200 {
		next.Records = next.Records[:200]
	}
	if err = s.commit(next); err != nil {
		return Record{}, err
	}
	return v, nil
}
func (s *Store) Records() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Record{}, s.data.Records...)
}
