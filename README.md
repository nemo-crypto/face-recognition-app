# Face Studio · 人脸核验全栈 Demo

新的主线由 **TypeScript 响应式 Web + Go HTTP 后端** 组成，支持人员登记、摄像头/选图采集、单次核验、记录筛选和关联数据删除。默认完全在本地运行，不依赖旧业务服务器或 H5+ 容器。

**默认模式只演示流程，不执行人脸识别或活体检测，不生成相似度。** 可接入符合协议的真实模型服务，但仓库不包含人脸模型权重、推理引擎或准确率验证。只有服务返回完整且有效的单人脸、活体和比对结果时，模型模式才可能显示通过。

旧 `Demo/` 保留为历史参考，不是新入口，Go 服务不会对外提供它或旧 APK。

<!-- BEGIN project-discovery -->
## 项目定位与实践入口

TypeScript + Go face verification workflow demo with camera capture, photo upload and a model-provider API. 人脸核验全栈教学，默认模拟流程，不含识别模型。

适合从以下入口阅读实现与接入条件：

- [Go 后端](backend/)
- [Web 采集界面](web/)
- [架构与模型接入边界](docs/architecture.md)
<!-- END project-discovery -->

## 快速启动

环境：Node.js 22+（推荐 24）、Go 1.22+；后端进程锁支持 macOS/Linux/FreeBSD，Windows 可使用 WSL。

```sh
npm ci
npm run build
go run ./backend/cmd/server
```

打开 **http://127.0.0.1:8080**。前端和 API 由同一个 Go 进程提供，不需要另启 Web 服务器。

不使用真实照片也能体验：切换“人员登记” → 点击“使用示例图” → 输入演示名称并确认授权 → 保存 → 确认本次核验授权 → 开始核验 → 查看记录 → 删除人员。示例图由浏览器本地绘制，不是真实人物。

## 功能与适配

- 三个工作区：人脸核验、人员登记、核验记录；提供加载、空状态、失败提示和删除确认。
- 桌面侧边栏、平板紧凑导航、手机顶部导航；支持 360px 起布局与安全区，输入框避免 iOS 自动放大。
- 使用现代 `getUserMedia`、`srcObject`、`playsinline`；允许切换前后摄像头，离开页面或切到后台时释放媒体轨道。
- 相机由用户主动开启，照片只在点击提交后上传；摄像头权限拒绝、无摄像头或环境不安全时可改用选图。
- 接受不超过 3MB 的 JPEG/PNG。图片最小 96×96、最大边长 4096、总像素最多 1200 万。浏览器等比例缩至最长边 1024；后端独立校验并重新编码，去除 EXIF 等元数据。
- 本地参考照、人员和结果持久化；临时核验照片不落盘。删除人员同时删除参考照及关联记录。

## 开发与验证

```sh
# 后端（保留运行）
go run ./backend/cmd/server
# 另一个终端：Vite 开发服务会代理 /api 到 Go
npm run dev

# 前端生产构建、Go 竞态测试、静态检查
npm run check

# 浏览器测试（自动启动隔离数据目录的 Go 服务）
npx playwright install chromium
npm run test:e2e
```

浏览器测试使用 Chromium 桌面与移动设备模拟，覆盖完整业务流程、失败处理和 360/390/768/1024/1440px 布局。真实 iOS Safari、Android 相机硬件、HEIC 解码和远程模型效果不在自动化验证范围内；HEIC 需先转 JPEG/PNG。

## 后端配置

| 配置 | 默认 | 作用 |
| --- | --- | --- |
| `-addr` | 127.0.0.1:8080 | HTTP 监听地址 |
| `-data` | data | 本地状态与锁文件目录 |
| `-web` | web/dist | 前端生产资源目录 |
| `FACE_API_TOKEN` | 空 | API Bearer token；非回环监听要求至少 24 字符 |
| `FACE_PROVIDER_URL` | 空 | 设置后启用远程模型，指向完整核验 endpoint |
| `FACE_PROVIDER_TOKEN` | 空 | 仅服务端使用的模型服务令牌 |
| `FACE_THRESHOLD` | 0.8 | 模型模式阈值，0..1，需按具体模型校准 |

令牌在页面“连接设置”输入，仅保存在当前页面内存中，不写入 LocalStorage。环境变量不会自动从 `.env` 读取，请通过 shell 或进程管理器注入。

## 数据与运行边界

这是面向本地开发的 Demo，不是门禁、考勤或生产身份认证系统。默认没有用户账号、角色隔离或多租户。参考照片在 `data/state.json` 中以 Base64 形式保存，**不是加密存储**；文件默认 0600、目录 0700，运行前需获得照片使用授权。

最多保存 50 位人员及最近 200 条核验结果，单机单进程写入。服务使用进程锁与原子文件替换，写入失败不更新内存状态。删除会移除当前状态中的数据，但不承诺清除文件系统快照、外部备份或模型服务留存。请勿把数据目录提交到版本库。

手机通过局域网访问时，相机通常要求 HTTPS；直接访问 `http://局域网IP` 不能保证打开相机。部署方式见 [适配与部署](docs/deployment.md)。

## 文档

- [架构与后端设计](docs/architecture.md)
- [API 与模型接入协议](docs/api.md)
- [适配与部署方案](docs/deployment.md)
- [验收清单](docs/testing.md)
- [FigJam 架构图](https://www.figma.com/board/GqxdpbZKYdEcoMwr2Bc6Zp)
