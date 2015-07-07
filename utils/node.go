package utils

type FileNode struct {
	Parent string `json:"parent"`
	Name   string `json:"name"`
	Dir    bool   `json:"bool"`
	Size   int64  `json:"size"`
	Mode   int64  `json:"mode"`
}
