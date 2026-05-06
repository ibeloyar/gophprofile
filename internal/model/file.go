package model

type AvatarFile struct {
	Filename    string
	ContentType string
	Width       int
	Height      int
	Size        int64
	Data        []byte
}
