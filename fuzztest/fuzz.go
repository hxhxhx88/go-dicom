package fuzz

import (
	"bytes"

	"github.com/hxhxhx88/go-dicom"
)

func Fuzz(data []byte) int {
	_, _ = dicom.ReadDataSet(bytes.NewBuffer(data), int64(len(data)),
		dicom.ReadOptions{})
	return 1
}
