package scanner

import (
	"path"
	"strings"
)

// Extension allowlists, each mapped to its MIME type explicitly. mime.TypeByExtension isn't used
// because whether it knows video types depends on the host's mime.types file, so the answer
// would differ between a Mac and a Linux server.
var (
	videoTypes = map[string]string{
		".mp4":  "video/mp4",
		".mkv":  "video/x-matroska",
		".webm": "video/webm",
		".mov":  "video/quicktime",
	}
	imageTypes = map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".webp": "image/webp",
	}
)

// ext returns name's extension, lowercased: "Page-1.PNG" → ".png".
func ext(name string) string {
	return strings.ToLower(path.Ext(name))
}

// hidden reports whether name is a dotfile such as .DS_Store or a macOS ._ resource fork.
func hidden(name string) bool {
	return strings.HasPrefix(name, ".")
}
