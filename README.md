# MUI / H5+ 任务与人脸核验界面示例

实际应用位于 [Demo/](Demo/README.md)，使用MUI、原生HTML/JavaScript与H5+设备接口，包含任务准备、人员与工具确认、定位、拍照上传和人脸核验界面。

当前仓库提供客户端页面和业务请求，原说明中的“人脸识别”为模拟/接入示例，不包含人脸模型训练、推理服务或识别精度验证。

## 开始阅读

| 入口 | 用途 |
|---|---|
| [Demo/README.md](Demo/README.md) | 实际应用的运行、配置与能力说明 |
| [Demo/manifest.json](Demo/manifest.json) | H5 App启动页与设备配置 |
| [登录页面](Demo/taskPrepareStage/login.html) | manifest中的启动入口 |
| [公共脚本](Demo/js/common.js) | 服务地址、用户上下文和任务公共请求 |

普通页面预览可在本目录执行：

```bash
python3 -m http.server 8080 --bind 127.0.0.1
```

打开 `http://127.0.0.1:8080/Demo/taskPrepareStage/login.html`。相机、原生定位和WebView能力需要在相应App容器及设备上验证。

## 项目边界

没有package.json、独立前端打包脚本或配套业务后端。历史服务地址需要按实际环境更新；登录、任务、图片上传与核验结果均需接口支持。设备打包、签名和识别效果没有在本轮验证。
