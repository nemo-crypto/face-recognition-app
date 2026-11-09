import './style.css';

type Subject = { id: string; name: string; created_at: string };
type RecordItem = { id: string; subject_id: string; name: string; created_at: string; mode: string; decision: string; score?: number; message: string };
type Health = { status: string; mode: 'demo' | 'remote'; auth_required: boolean };
type View = 'verify' | 'enroll' | 'history';
const $ = <T extends HTMLElement = HTMLElement>(id: string) => document.getElementById(id) as T;
const escapeHTML = (value: string) => value.replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]!));
const date = (value: string) => new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
const labels: { [key: string]: string } = { demo: '演示完成', matched: '模型通过', not_matched: '未匹配', review: '需复核' };
let health: Health | null = null;
let subjects: Subject[] = [];
let records: RecordItem[] = [];
let view: View = 'verify';
let photo = '';
let token = '';
let busy = false;
let stream: MediaStream | null = null;
let cameraGeneration = 0;
let facing: 'user' | 'environment' = 'user';
let deleteID = '';

$('app').innerHTML = `
<a class="skip" href="#main">跳至主要内容</a>
<aside class="sidebar">
  <a class="brand" href="/" aria-label="Face Studio 首页"><span class="brand-icon">F<span>·</span></span><span>Face Studio<small>IDENTITY WORKSPACE</small></span></a>
  <div class="sidebar-label">工作空间</div>
  <nav aria-label="主导航">
    <button class="nav-item active" data-view="verify" aria-current="page"><span class="nav-icon">◎</span>人脸核验<span class="nav-arrow">↗</span></button>
    <button class="nav-item" data-view="enroll"><span class="nav-icon">⊕</span>人员登记</button>
    <button class="nav-item" data-view="history"><span class="nav-icon">▤</span>核验记录<span id="nav-count" class="count">0</span></button>
  </nav>
  <div class="sidebar-bottom"><div class="local-indicator"><i></i>本地优先 · 数据可删除</div><p>每一次核验，都由你主动发起。</p><span class="sidebar-help">Face Studio · 本地工作区</span></div>
</aside>
<div class="shell">
  <header class="topbar"><div class="breadcrumb">工作空间 <span>/</span> <strong id="crumb">人脸核验</strong></div><div class="topbar-right"><span id="connection" class="connection">连接中</span><button id="settings-open" class="text-button settings-button">连接设置</button><span class="avatar" aria-hidden="true">D</span></div></header>
  <main id="main" tabindex="-1">
    <div class="heading"><div><div class="eyebrow">FACE VERIFICATION</div><h1 id="page-title">让每一次核验，更清晰。</h1><p id="page-desc">选择人员，采集照片，查看完整的核验结果。</p></div><span id="mode-badge" class="mode-badge">正在连接服务</span></div>
    <div id="notice" class="notice"><span aria-hidden="true">ⓘ</span><p id="mode-description">正在确认服务模式，请稍候。</p></div>
    <div id="alert" class="alert" role="alert" hidden></div>
    <section class="stats" aria-label="工作区概览"><div><span>已登记人员</span><strong id="subject-count">0<small> 人</small></strong><i class="stat-icon">♙</i></div><div><span>核验记录</span><strong id="record-count">0<small> 条</small></strong><i class="stat-icon">▤</i></div><div><span>当前运行模式</span><strong id="mode-text" class="stat-text">连接中</strong><i class="stat-icon">◎</i></div></section>
    <div id="workflow" class="workspace">
      <section class="card capture-card" aria-labelledby="capture-title"><div class="card-header"><div><h2 id="capture-title">照片采集</h2><p>画面清晰、正对镜头，一次只采集一人</p></div><span class="step">01</span></div>
        <div class="capture-stage" id="capture-stage">
          <div class="capture-grid"></div><video id="video" autoplay playsinline muted hidden></video><img id="preview" alt="本次采集的照片预览" hidden />
          <div id="camera-empty" class="camera-empty"><div class="face-frame"><span class="face-outline"><i></i></span></div><h3>准备好，开始采集</h3><p>开启摄像头，或从设备选择一张照片</p></div>
          <span class="capture-tag" id="capture-tag">待采集</span><span class="stage-note">照片仅在你点击提交后发送</span>
        </div>
        <div class="capture-actions"><button id="camera-start" class="button primary">◎ 开启摄像头</button><button id="capture" class="button primary" disabled hidden>拍摄照片</button><button id="switch-camera" class="button secondary" hidden>切换镜头</button><button id="upload" class="button secondary">↥ 上传照片</button><input id="file" type="file" accept="image/jpeg,image/png" hidden /><button id="reset-photo" class="text-button" hidden>重新采集</button></div>
        <div class="capture-footer"><span>JPEG / PNG · 最大 3 MB</span><button id="sample" class="text-button">使用示例图 ↗</button></div>
      </section>
      <aside class="side-panels">
        <section id="verify-panel" class="card action-card"><div class="card-header"><div><h2>发起核验</h2><p>与已登记人员进行一对一比对</p></div><span class="step">02</span></div>
          <form id="verify-form"><label for="subject">选择核验人员</label><select id="subject" required><option value="">请先登记人员</option></select><button type="button" class="text-button add-person" data-view="enroll">＋ 登记新人员</button>
          <label class="consent"><input id="verify-consent" type="checkbox" required /><span>我已获得图片使用授权，同意提交本次照片用于核验。</span></label><button id="verify-submit" class="button primary full" disabled>开始核验 <span>→</span></button></form>
          <div class="private-note">核验照片仅用于本次处理，不保存到记录。</div>
        </section>
        <section id="enroll-panel" class="card action-card" hidden><div class="card-header"><div><h2>登记新人员</h2><p>使用左侧照片作为登记参考照</p></div><span class="step">02</span></div>
          <form id="enroll-form"><label for="person-name">姓名或演示名称</label><input id="person-name" placeholder="例如：演示人员 A" maxlength="50" autocomplete="off" required /><label class="consent"><input id="enroll-consent" type="checkbox" required /><span>我已获得授权，同意在本地保存参考照；可随时删除人员及关联记录。</span></label><button id="enroll-submit" class="button primary full" disabled>保存人员 <span>＋</span></button></form>
        </section>
        <section id="result-panel" class="card result-card" aria-live="polite"><div class="result-symbol">✓</div><h3 id="result-title">等待本次结果</h3><p id="result-message">完成采集后，核验结果会显示在这里。</p><span id="result-detail"></span></section>
        <section class="tip-card"><span>✧</span><div><h3>一个更好的采集环境</h3><p>保持光线均匀，避免逆光与遮挡。若摄像头不可用，可以上传照片继续体验。</p></div></section>
      </aside>
    </div>
    <section id="people-section" class="card people-card" hidden><div class="card-header"><div><h2>人员列表</h2><p>删除人员会同时清除参考照和关联核验记录</p></div><span id="people-total" class="subtle"></span></div><div id="people-list"></div></section>
    <section id="history-panel" class="card history-card" hidden><div class="card-header"><div><h2>核验记录</h2><p>仅保留最近 200 条结果，不保存本次采集照片</p></div><button id="refresh" class="button secondary small">刷新</button></div><div class="history-filters"><input id="search" type="search" placeholder="搜索人员名称" aria-label="搜索人员名称" /><select id="filter" aria-label="筛选核验结果"><option value="">全部结果</option><option value="demo">演示完成</option><option value="matched">模型通过</option><option value="not_matched">未匹配</option><option value="review">需复核</option></select></div><div id="record-list"></div></section>
    <footer class="page-footer"><span>Face Studio / Demo workspace</span><span>主动采集 · 明确结果 · 本地可控</span></footer>
  </main>
</div>
<div id="toast" role="status" class="toast" hidden></div>
<dialog id="settings"><form id="settings-form"><div class="dialog-header"><h2>连接设置</h2><button type="button" class="icon-button" id="settings-close" aria-label="关闭连接设置">×</button></div><p>远程部署需要访问令牌。本地默认模式可以留空，令牌仅保存在当前页面内存中。</p><label for="token">访问令牌</label><input id="token" type="password" autocomplete="off" placeholder="输入服务端配置的令牌" /><button class="button primary full" type="submit">连接服务</button></form></dialog>
<dialog id="delete-dialog"><h2>删除这位人员？</h2><p>参考照片和该人员的核验记录会一起删除，无法在页面内恢复。</p><div class="dialog-actions"><button id="delete-cancel" class="button secondary">取消</button><button id="delete-confirm" class="button danger">确认删除</button></div></dialog>`;

function notify(message: string) { $('toast').textContent = message; $('toast').hidden = false; window.setTimeout(() => { $('toast').hidden = true; }, 3500); }
function showError(error: unknown) { $('alert').textContent = error instanceof Error ? error.message : '操作失败，请稍后重试'; $('alert').hidden = false; }
function clearError() { $('alert').hidden = true; }
async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const controller = new AbortController();
  const timer = window.setTimeout(() => controller.abort(), 22000);
  try {
    const response = await fetch(`/api${path}`, { ...options, signal: controller.signal, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers } });
    if (response.status === 204) return undefined as T;
    const result = await response.json().catch(() => ({})) as { error?: string };
    if (!response.ok) throw new Error(result.error ?? `请求失败 (${response.status})`);
    return result as T;
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw new Error('请求超时。请先刷新记录确认结果，再决定是否重试。');
    if (error instanceof TypeError) throw new Error('无法连接服务，请检查网络与 Go 后端状态。');
    throw error;
  } finally { window.clearTimeout(timer); }
}
function updateControls() {
  const ready = !!health && !!photo && !busy;
  $<HTMLButtonElement>('verify-submit').disabled = !ready || !$<HTMLSelectElement>('subject').value || !$<HTMLInputElement>('verify-consent').checked;
  $<HTMLButtonElement>('enroll-submit').disabled = !ready || !$<HTMLInputElement>('person-name').value.trim() || !$<HTMLInputElement>('enroll-consent').checked;
  for (const id of ['camera-start', 'upload', 'sample', 'reset-photo', 'switch-camera', 'subject', 'person-name', 'verify-consent', 'enroll-consent']) ($<HTMLButtonElement>(id)).disabled = busy;
  document.querySelectorAll<HTMLButtonElement>('[data-view], [data-delete]').forEach((button) => { button.disabled = busy; });
  $('verify-submit').textContent = busy && view === 'verify' ? '正在核验…' : '开始核验 →';
  $('enroll-submit').textContent = busy && view === 'enroll' ? '正在保存…' : '保存人员 ＋';
}
async function action(run: () => Promise<void>) { if (busy) return; busy = true; clearError(); updateControls(); try { await run(); } catch (error) { showError(error); } finally { busy = false; updateControls(); } }
function switchView(next: View) {
  view = next; stopCamera(); clearError();
  document.querySelectorAll<HTMLButtonElement>('nav [data-view]').forEach((button) => { const active = button.dataset.view === next; button.classList.toggle('active', active); if (active) button.setAttribute('aria-current', 'page'); else button.removeAttribute('aria-current'); });
  $('workflow').hidden = next === 'history'; $('history-panel').hidden = next !== 'history'; $('people-section').hidden = next !== 'enroll';
  $('verify-panel').hidden = next !== 'verify'; $('enroll-panel').hidden = next !== 'enroll';
  const titles = { verify: ['人脸核验', '让每一次核验，更清晰。', '选择人员，采集照片，查看完整的核验结果。'], enroll: ['人员登记', '从一份清晰的登记开始。', '建立参考照片，人员与数据都由你管理。'], history: ['核验记录', '每一次操作，都有迹可循。', '查看最近的核验结果，区分演示记录与模型结果。'] };
  const [crumb, title, desc] = titles[next]; $('crumb').textContent = crumb!; $('page-title').textContent = title!; $('page-desc').textContent = desc!;
  updateControls();
}
function render() {
  const selected = $<HTMLSelectElement>('subject').value;
  $('subject').innerHTML = '<option value="">' + (subjects.length ? '请选择一位人员' : '请先登记人员') + '</option>' + subjects.map((p) => `<option value="${escapeHTML(p.id)}">${escapeHTML(p.name)}</option>`).join('');
  if (subjects.some((person) => person.id === selected)) $<HTMLSelectElement>('subject').value = selected;
  $('subject-count').innerHTML = `${subjects.length}<small> 人</small>`; $('record-count').innerHTML = `${records.length}<small> 条</small>`; $('nav-count').textContent = String(records.length); $('people-total').textContent = `${subjects.length} / 50 人`;
  $('people-list').innerHTML = subjects.length ? subjects.map((p) => `<div class="person-row"><span class="person-avatar">${escapeHTML(p.name.slice(0, 1))}</span><div><strong>${escapeHTML(p.name)}</strong><small>登记于 ${date(p.created_at)}</small></div><button class="text-button delete-button" data-delete="${escapeHTML(p.id)}" aria-label="删除 ${escapeHTML(p.name)}">删除</button></div>`).join('') : '<div class="empty-state">还没有人员。使用示例图即可完成第一次登记。</div>';
  renderRecords(); updateControls();
}
function renderRecords() {
  const search = $<HTMLInputElement>('search').value.trim().toLowerCase(); const filter = $<HTMLSelectElement>('filter').value;
  const matches = records.filter((r) => r.name.toLowerCase().includes(search) && (!filter || r.decision === filter));
  $('record-list').innerHTML = matches.length ? matches.map((r) => `<article class="record-row"><span class="person-avatar">${escapeHTML(r.name.slice(0, 1))}</span><div class="record-main"><strong>${escapeHTML(r.name)}</strong><p>${escapeHTML(r.message)}</p><small>${date(r.created_at)} · ${r.mode === 'demo' ? '演示模式' : '模型模式'}${r.score === undefined ? '' : ` · 模型分数 ${r.score.toFixed(3)}`}</small></div><span class="decision ${r.decision === 'matched' ? 'positive' : ''}">${escapeHTML(labels[r.decision] ?? r.decision)}</span></article>`).join('') : '<div class="empty-state"><span>▤</span><h3>暂无符合条件的记录</h3><p>完成一次核验后，结果会出现在这里。</p></div>';
}
async function reload() { [subjects, records] = await Promise.all([api<Subject[]>('/subjects'), api<RecordItem[]>('/records')]); render(); }
async function connect() {
  clearError();
  try {
    health = await api<Health>('/health');
    $('connection').textContent = '服务已连接'; $('connection').classList.add('online');
    $('mode-badge').textContent = health.mode === 'demo' ? '◌ 演示模式' : '◎ 模型服务'; $('mode-text').textContent = health.mode === 'demo' ? '演示模式' : '模型服务';
    $('mode-description').textContent = health.mode === 'demo' ? '当前为演示模式：可以体验完整流程，但不会执行身份识别或活体检测，也不会生成相似度分数。' : '当前连接真实模型服务。比对结果需同时满足单人脸、活体与阈值要求；必要时请人工复核。';
    $('sample').hidden = health.mode !== 'demo'; await reload();
  } catch (error) { health = null; subjects = []; records = []; render(); $('connection').textContent = '连接需检查'; $('connection').classList.remove('online'); showError(error); }
  updateControls();
}
function stopCamera() {
  cameraGeneration++; stream?.getTracks().forEach((track) => track.stop()); stream = null;
  const video = $<HTMLVideoElement>('video'); video.srcObject = null; video.hidden = true;
  $('capture').hidden = true; $('switch-camera').hidden = true; $('camera-start').hidden = false;
  $('camera-empty').hidden = !!photo; $('capture-tag').textContent = photo ? '已采集' : '待采集';
}
function setPhoto(value: string) { stopCamera(); photo = value; const preview = $<HTMLImageElement>('preview'); preview.src = value; preview.hidden = false; $('camera-empty').hidden = true; $('reset-photo').hidden = false; $('capture-tag').textContent = '已采集 · 可提交'; updateControls(); }
function resetPhoto() { stopCamera(); photo = ''; const preview = $<HTMLImageElement>('preview'); preview.removeAttribute('src'); preview.hidden = true; $('camera-empty').hidden = false; $('reset-photo').hidden = true; $('capture-tag').textContent = '待采集'; $<HTMLInputElement>('file').value = ''; updateControls(); }
async function startCamera() {
  clearError(); stopCamera(); const generation = cameraGeneration;
  if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) { showError(new Error('摄像头需要 HTTPS 或 localhost 安全环境；也可以选择上传照片。')); return; }
  try {
    const media = await navigator.mediaDevices.getUserMedia({ audio: false, video: { facingMode: { ideal: facing }, width: { ideal: 1280 }, height: { ideal: 960 } } });
    if (generation !== cameraGeneration) { media.getTracks().forEach((track) => track.stop()); return; }
    stream = media; const video = $<HTMLVideoElement>('video'); video.srcObject = media; video.hidden = false; video.classList.toggle('mirrored', facing === 'user');
    $('preview').hidden = true; $('camera-empty').hidden = true; $('camera-start').hidden = true; $('capture').hidden = false; $('switch-camera').hidden = false; $('capture-tag').textContent = '摄像头已开启';
    photo = ''; $('reset-photo').hidden = true; $<HTMLButtonElement>('capture').disabled = true; updateControls(); await video.play();
    if (generation === cameraGeneration) $<HTMLButtonElement>('capture').disabled = false;
  } catch (error) {
    if (generation !== cameraGeneration) return; stopCamera();
    const name = error instanceof DOMException ? error.name : '';
    showError(new Error(name === 'NotAllowedError' ? '摄像头权限被拒绝。请在浏览器设置中允许访问，或上传照片。' : name === 'NotFoundError' ? '未找到可用摄像头，请使用上传照片。' : '摄像头暂时不可用，可能正被其他应用占用。请重试或上传照片。'));
  }
}
function capture() {
  const video = $<HTMLVideoElement>('video'); if (!video.videoWidth || !video.videoHeight) { showError(new Error('摄像头画面尚未就绪，请稍候重试')); return; }
  const canvas = document.createElement('canvas'); const scale = Math.min(1, 1024 / Math.max(video.videoWidth, video.videoHeight)); canvas.width = Math.round(video.videoWidth * scale); canvas.height = Math.round(video.videoHeight * scale);
  canvas.getContext('2d')!.drawImage(video, 0, 0, canvas.width, canvas.height); setPhoto(canvas.toDataURL('image/jpeg', 0.85));
}
async function upload(file: File) {
  if (!['image/jpeg', 'image/png'].includes(file.type) || file.size > 3 * 1024 * 1024) throw new Error('请选择不超过 3MB 的 JPEG 或 PNG 图片。HEIC 请先转换格式。');
  const url = URL.createObjectURL(file);
  try {
    const image = new Image(); image.src = url; await image.decode();
    if (Math.min(image.naturalWidth, image.naturalHeight) < 96 || image.naturalWidth > 4096 || image.naturalHeight > 4096 || image.naturalWidth * image.naturalHeight > 12000000) throw new Error('图片最小 96×96，最大边长 4096，且不超过 1200 万像素。');
    const scale = Math.min(1, 1024 / Math.max(image.naturalWidth, image.naturalHeight)); const canvas = document.createElement('canvas'); canvas.width = Math.round(image.naturalWidth * scale); canvas.height = Math.round(image.naturalHeight * scale);
    const context = canvas.getContext('2d')!; context.fillStyle = '#ffffff'; context.fillRect(0, 0, canvas.width, canvas.height); context.drawImage(image, 0, 0, canvas.width, canvas.height); setPhoto(canvas.toDataURL('image/jpeg', 0.85));
  } finally { URL.revokeObjectURL(url); }
}
function sample() {
  const canvas = document.createElement('canvas'); canvas.width = 640; canvas.height = 480; const ctx = canvas.getContext('2d')!;
  const gradient = ctx.createLinearGradient(0, 0, 640, 480); gradient.addColorStop(0, '#dce9f4'); gradient.addColorStop(1, '#c8bff3'); ctx.fillStyle = gradient; ctx.fillRect(0, 0, 640, 480);
  ctx.fillStyle = '#79729b'; ctx.beginPath(); ctx.ellipse(320, 420, 130, 120, 0, 0, Math.PI * 2); ctx.fill(); ctx.fillStyle = '#f5f3fc'; ctx.beginPath(); ctx.ellipse(320, 205, 80, 100, 0, 0, Math.PI * 2); ctx.fill();
  ctx.fillStyle = '#79729b'; ctx.beginPath(); ctx.arc(290, 200, 6, 0, Math.PI * 2); ctx.arc(350, 200, 6, 0, Math.PI * 2); ctx.fill(); ctx.strokeStyle = '#79729b'; ctx.lineWidth = 4; ctx.beginPath(); ctx.arc(320, 225, 22, 0.2, Math.PI - 0.2); ctx.stroke();
  ctx.font = '16px sans-serif'; ctx.fillStyle = '#454060'; ctx.fillText('DEMO IMAGE · NOT A REAL PERSON', 22, 35); setPhoto(canvas.toDataURL('image/jpeg', 0.85)); notify('已载入示例图，不涉及真实人员。');
}

document.querySelectorAll<HTMLButtonElement>('[data-view]').forEach((button) => button.addEventListener('click', () => switchView(button.dataset.view as View)));
$('camera-start').addEventListener('click', () => { void startCamera(); }); $('switch-camera').addEventListener('click', () => { facing = facing === 'user' ? 'environment' : 'user'; void startCamera(); });
$('capture').addEventListener('click', capture); $('reset-photo').addEventListener('click', resetPhoto); $('sample').addEventListener('click', sample); $('upload').addEventListener('click', () => $('file').click());
$('file').addEventListener('change', () => { const file = $<HTMLInputElement>('file').files?.[0]; if (file) void action(() => upload(file)); });
for (const id of ['subject', 'person-name', 'verify-consent', 'enroll-consent']) $(id).addEventListener('input', updateControls);
$('enroll-form').addEventListener('submit', (event) => { event.preventDefault(); void action(async () => {
  const person = await api<Subject>('/subjects', { method: 'POST', body: JSON.stringify({ name: $<HTMLInputElement>('person-name').value.trim(), image: photo, consent: $<HTMLInputElement>('enroll-consent').checked }) });
  $<HTMLInputElement>('person-name').value = ''; $<HTMLInputElement>('enroll-consent').checked = false; subjects = [...subjects, person]; render(); $<HTMLSelectElement>('subject').value = person.id; notify('登记成功，可以开始核验。'); switchView('verify');
}); });
$('verify-form').addEventListener('submit', (event) => { event.preventDefault(); void action(async () => {
  $('result-title').textContent = '正在处理照片'; $('result-message').textContent = '请稍候，不要重复提交。'; $('result-detail').textContent = '';
  try {
    const result = await api<RecordItem>('/verifications', { method: 'POST', body: JSON.stringify({ subject_id: $<HTMLSelectElement>('subject').value, image: photo, consent: $<HTMLInputElement>('verify-consent').checked }) });
    $('result-title').textContent = labels[result.decision] ?? result.decision; $('result-message').textContent = result.message; $('result-detail').textContent = result.score === undefined ? '未生成身份相似度分数' : `模型分数 ${result.score.toFixed(3)} · ${date(result.created_at)}`; $<HTMLInputElement>('verify-consent').checked = false; records = [result, ...records].slice(0, 200); render();
  } catch (error) { $('result-title').textContent = '本次操作未完成'; $('result-message').textContent = '请查看上方提示，检查后再试。'; throw error; }
}); });
$('people-list').addEventListener('click', (event) => { const button = (event.target as HTMLElement).closest<HTMLButtonElement>('[data-delete]'); if (button) { deleteID = button.dataset.delete!; $<HTMLDialogElement>('delete-dialog').showModal(); } });
$('delete-cancel').addEventListener('click', () => $<HTMLDialogElement>('delete-dialog').close());
$('delete-confirm').addEventListener('click', () => { $<HTMLDialogElement>('delete-dialog').close(); void action(async () => { await api(`/subjects/${encodeURIComponent(deleteID)}`, { method: 'DELETE' }); subjects = subjects.filter((person) => person.id !== deleteID); records = records.filter((record) => record.subject_id !== deleteID); resetPhoto(); render(); $('result-title').textContent = '等待本次结果'; $('result-message').textContent = '完成采集后，核验结果会显示在这里。'; $('result-detail').textContent = ''; notify('人员、参考照和关联记录已删除。'); }); });
$('search').addEventListener('input', renderRecords); $('filter').addEventListener('change', renderRecords); $('refresh').addEventListener('click', () => { void action(reload); });
$('settings-open').addEventListener('click', () => $<HTMLDialogElement>('settings').showModal()); $('settings-close').addEventListener('click', () => $<HTMLDialogElement>('settings').close());
$('settings-form').addEventListener('submit', (event) => { event.preventDefault(); token = $<HTMLInputElement>('token').value.trim(); $<HTMLInputElement>('token').value = ''; $<HTMLDialogElement>('settings').close(); void connect(); });
window.addEventListener('pagehide', stopCamera); document.addEventListener('visibilitychange', () => { if (document.hidden) stopCamera(); });
void connect();
