# 适配与部署

## 本地开发

`npm run build && go run ./backend/cmd/server` 提供单进程 Demo。开发前端时另开 `npm run dev`，Vite 同源代理 API，实际后端仍运行在 127.0.0.1:8080。

## 手机和平板

前端覆盖小屏、横屏、平板和桌面布局，使用 playsinline 避免 iOS 强制全屏。输入使用 16px 字号避免手机聚焦时自动缩放；没有禁止页面缩放。UI 提供键盘焦点、原生表单标签、状态消息和原生 dialog。

摄像头依赖安全上下文。localhost 通常允许相机；手机上的 localhost 指向手机自身，不是开发电脑。电脑的局域网 HTTP 地址通常不能访问 getUserMedia。手机联调应通过可信 HTTPS 域名/证书与反向代理访问，或暂时使用上传照片。

Chrome 移动模拟不能替代真机：上线前需验证 Safari/Chrome 的权限拒绝、前后镜头、后台恢复、横竖屏、低内存设备和不同照片格式。HTML file input 不承诺设备相册会自动转换 HEIC，所以界面明确只接受 JPEG/PNG。

## 同源 HTTPS 反向代理

推荐使用反向代理终止 TLS，转发所有路径到 Go，保留 Host 和 Origin。示意 Caddy 配置：

```caddy
face-demo.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

使用自己的域名与有效证书，不直接复制示例域名。设置至少 24 字符的随机 `FACE_API_TOKEN`，即便 Go 仅监听回环地址，通过代理提供外部访问时也应设置。未设置令牌的本地模式会拒绝外部 Host。

把 Go 运行用户限制为数据目录拥有者，使用进程管理器注入环境变量。不要在命令日志、URL、前端源码或 Git 中写入令牌。请求体大小需允许最多 5MiB，代理超时应大于模型超时，禁用 API 缓存。

## 可部署产物

```sh
npm ci
npm run build
go build -o server ./backend/cmd/server
./server -addr 127.0.0.1:8080 -web web/dist -data /absolute/path/face-data
```

部署需要 server 与 web/dist，数据目录独立保存。不要部署仓库根目录为静态站点，否则可能暴露历史 Demo、APK 或其他文件。当前 Go 仅提供 `/` 与 `/assets/` 的前端构建内容。

容器化或多副本部署之前，先将 JSON 存储迁移为事务数据库，并为图片加密、备份和删除制定明确策略。当前示例没有提供虚假的多实例支持。
