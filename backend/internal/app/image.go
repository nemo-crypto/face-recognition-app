package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"strings"
)

// 先检查尺寸再解码，拒绝伪装文件和过大图片；重编码去除 EXIF 元数据。
func normalizeImage(data string) (string, error) {
	const limit = 3 << 20
	if strings.HasPrefix(data, "data:") {
		parts := strings.SplitN(data, ",", 2)
		if len(parts) != 2 || (parts[0] != "data:image/jpeg;base64" && parts[0] != "data:image/png;base64") {
			return "", errors.New("仅支持 JPEG 或 PNG 图片")
		}
		data = parts[1]
	}
	if len(data) > base64.StdEncoding.EncodedLen(limit) {
		return "", errors.New("图片不能超过 3MB")
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", errors.New("图片编码无效")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") {
		return "", errors.New("无法识别图片，请使用 JPEG 或 PNG")
	}
	if config.Width < 96 || config.Height < 96 || config.Width > 4096 || config.Height > 4096 || int64(config.Width)*int64(config.Height) > 12000000 {
		return "", errors.New("图片最小 96×96，最大边长 4096，且不超过 1200 万像素")
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", errors.New("图片解码失败")
	}
	var out bytes.Buffer
	if err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", errors.New("图片处理失败")
	}
	if out.Len() > limit {
		return "", errors.New("图片处理后超过 3MB，请缩小图片")
	}
	return base64.StdEncoding.EncodeToString(out.Bytes()), nil
}
