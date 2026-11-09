# API 与模型协议

所有 API 同源，以 JSON 传输。配置 FACE_API_TOKEN 后，除 health 外均要求 `Authorization: Bearer <token>`。错误统一返回 `{ "error": "面向用户的描述" }`。

| 方法与路径 | 请求 | 成功响应 |
| --- | --- | --- |
| GET /api/health | 无 | 200：status、mode、auth_required |
| GET /api/subjects | 无 | 200：人员数组，不包含参考照 |
| POST /api/subjects | name、image、consent | 201：人员 DTO |
| DELETE /api/subjects/{id} | 无 | 204：删除人员及关联记录 |
| POST /api/verifications | subject_id、image、consent | 201：核验记录 |
| GET /api/records | 无 | 200：最近 200 条，按新到旧 |

## 人员登记

```json
{
  "name": "演示人员 A",
  "image": "data:image/jpeg;base64,<JPEG数据>",
  "consent": true
}
```

name 去首尾空白后为 1–50 个字符，不允许控制字符。image 接受 JPEG/PNG 的 Base64 或对应 data URL；不接受 SVG、GIF 或远程图片 URL。consent 必须为 true。登记生成随机 id，相同姓名允许存在。

```json
{
  "id": "随机标识",
  "name": "演示人员 A",
  "created_at": "2026-11-09T10:00:00Z"
}
```

## 单次核验

```json
{
  "subject_id": "已登记人员标识",
  "image": "data:image/jpeg;base64,<本次照片>",
  "consent": true
}
```

```json
{
  "id": "记录标识",
  "subject_id": "人员标识",
  "name": "演示人员 A",
  "created_at": "2026-11-09T10:01:00Z",
  "mode": "demo",
  "decision": "demo",
  "message": "演示流程已完成，未进行身份识别或活体检测。"
}
```

decision：demo / matched / not_matched / review。demo 永远不包含 score。remote 在可比对情况下可以包含 0..1 的模型分数；review 不提供可信比对分数。接口返回成功代表记录已保存，不代表身份通过。

## 状态码

400 输入或图片无效、缺少授权确认；401 令牌错误；403 来源/Host 不允许；404 接口或人员不存在；409 人员容量已满；413 请求体超过限制；415 Content-Type 不支持；429 图片处理并发已满；500 持久化失败；502 模型服务失败或协议不合格；503 前端未构建。

核验失败的 HTTP 请求不会生成成功记录；不存在自动通过兜底。服务不保证请求幂等，调用方不要在超时后盲目重发。

## 模型服务适配

`FACE_PROVIDER_URL` 是完整的 POST endpoint，例如本机 `http://127.0.0.1:9000/verify`。远程部署请使用 HTTPS 和模型服务令牌。Go 请求：

```json
{
  "reference_image": "<重编码后的JPEG Base64，不含data前缀>",
  "probe_image": "<本次JPEG Base64，不含data前缀>"
}
```

模型服务必须对参考图与采集图执行必要的人脸检测、质量校验和比对，返回：

```json
{
  "matched": true,
  "score": 0.91,
  "face_count": 1,
  "liveness": "passed"
}
```

face_count 为本次采集图的人脸数量；参考图无人脸、多脸或无效时模型应返回非通过结果或错误，不能给出成功匹配。liveness 只接受 passed、failed、unknown；服务无法可靠判断活体时应返回 unknown。不得为了让界面变绿而固定返回 passed。

provider token 仅放在 Go 请求的 Authorization Header，浏览器无法读取。超时、非 200、字段缺失、非法分数或无效 JSON 均返回 502，不向前端透传内部错误信息。
