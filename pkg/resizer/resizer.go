package resizer

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/disintegration/imaging"
)

func Resize(imageData []byte, width, height int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	resized := imaging.Resize(img, width, height, imaging.Lanczos)

	buf := new(bytes.Buffer)

	err = jpeg.Encode(buf, resized, &jpeg.Options{Quality: 85})
	if err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), nil
}
