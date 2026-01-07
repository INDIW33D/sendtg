package entity

// FileInfo represents information about a file to send
type FileInfo struct {
	Path string
	Name string
	Size int64
}

// UploadProgress represents file upload progress
type UploadProgress struct {
	Uploaded int64   // Bytes uploaded
	Total    int64   // Total bytes
	Speed    float64 // Bytes per second
	Percent  float64 // 0-100
}
