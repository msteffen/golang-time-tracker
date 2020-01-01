// Code generated for package watchd by go-bindata DO NOT EDIT. (@generated)
// sources:
// templates/viz.html
package watchd

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _templatesVizHtml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xd4\x54\x4d\x73\x9b\x30\x10\xbd\xeb\x57\x6c\xe8\x74\x62\x4f\x0b\xb8\x49\x33\x93\xda\xc0\xa1\x69\x6e\x4d\x7b\xe9\xbd\x23\xd0\x1a\x94\x08\x89\x91\xd6\x7c\xd4\xe3\xff\xde\x11\x90\xd8\x6e\xf2\x07\x7a\x41\xec\xd3\xd3\xee\xbe\xb7\x88\xa4\xa2\x5a\x65\x2c\xa9\x90\x8b\x8c\x25\x17\x61\xc8\x7e\x55\xd2\x41\x61\x04\x02\x57\xca\x74\x0e\xb6\xc6\x82\x45\xee\x8c\xe6\xb9\x42\x70\xf2\x8f\xd4\x25\x74\x15\x6a\xa0\x0a\x99\xde\xd5\x39\x5a\x30\x5b\x20\x59\xa3\x75\x20\x1d\x5c\xa4\x70\xc3\xc2\x30\x63\x89\x2b\xac\x6c\x08\x68\x68\x30\x0d\x08\x7b\x8a\x1f\x79\xcb\x27\x34\xc8\x18\x80\xe0\x83\x83\x14\xf6\xfb\xe8\x70\x60\x00\x9d\xd4\xc2\x74\x11\x17\xe2\xbe\x45\x4d\xdf\xa5\x23\xd4\x68\x17\x97\xdf\x7e\x3e\xdc\x19\x4d\x1e\x33\x5c\xa0\xb8\xfc\x08\x0b\xf4\x94\x25\xa4\x19\xec\x19\x00\x80\x42\x02\xf2\xd9\x84\x29\x76\x35\x6a\x8a\x4a\xa4\x7b\x85\xfe\xd5\x7d\x1d\xee\x14\x77\xee\x07\xaf\x71\x11\x8c\xad\x06\xcb\xcd\xcb\xb1\x1e\x52\x78\xe0\x54\x45\x5b\x65\x8c\x5d\xdc\xae\x62\x72\x91\x42\x5d\x52\x35\xb3\xbc\x0d\x0b\x1a\x75\xba\xe5\x5c\x10\x80\x22\x47\x83\xc2\xa8\x93\x82\x2a\x48\xa1\x87\x0f\x10\xb4\x5d\xb0\xf9\x67\xbf\x42\x59\x56\xf4\x8a\xe0\x25\x1f\x96\x2c\x89\x27\x47\xbc\x61\x9e\x7e\xea\x57\xe1\x5c\x90\xb1\xdc\x88\x61\x2c\x2a\xa4\x6b\x14\x1f\xd6\xb0\x55\xd8\xfb\x24\x35\xb7\xa5\xd4\x6b\x58\x6d\xd8\x81\x45\xa3\xb0\xdf\x0d\x17\xc2\x0f\x69\x7f\xb2\xcf\x77\x64\xc6\xc7\x66\xb4\x59\x50\xb5\x86\x4f\xab\xd5\x7b\x1f\x4e\xdd\x4d\xf1\x31\x4b\x6e\xfa\x31\x43\xce\x8b\xa7\xd2\x9a\x9d\x16\x61\x61\x94\xb1\x6b\x50\x9e\x5e\x5a\x3e\x6c\xde\xea\xc8\xaf\x61\x67\x79\xb3\x06\x6d\xfc\xea\xc1\xc7\x9d\x23\xb9\x1d\xc2\x62\x1a\xe2\x1a\x0a\xd4\x84\xf6\xd8\xf4\x59\xb3\x57\xab\x66\x4c\xf5\xba\xf4\xbb\xeb\xdb\x2f\x9f\x4f\x25\xdc\xb4\xdd\x99\x84\x39\xae\xa5\x0e\x5f\x28\x73\x36\x8f\x1d\x79\xcf\x20\xef\x9f\x89\x57\xab\x13\xec\x99\x38\x83\x07\x3f\x24\x3f\x9b\x8c\x25\xf1\x7c\x5d\xfc\x50\x32\x96\x08\xd9\x42\xe1\xbf\xad\x34\x38\xb3\x3f\xc8\x58\x52\x73\xa9\xcf\x37\x73\xd3\x07\x6f\x1c\x0a\x32\xf0\xd7\x0f\xc4\x35\x14\xca\x14\x4f\x10\x86\x19\x24\xb1\x90\xed\x7f\x4a\x8e\xbd\x74\xbf\xce\xe1\x6c\x56\x3c\xfd\x71\xfe\x06\x00\x00\xff\xff\xed\x65\x82\x77\x79\x04\x00\x00")

func templatesVizHtmlBytes() ([]byte, error) {
	return bindataRead(
		_templatesVizHtml,
		"templates/viz.html",
	)
}

func templatesVizHtml() (*asset, error) {
	bytes, err := templatesVizHtmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "templates/viz.html", size: 1145, mode: os.FileMode(420), modTime: time.Unix(1577899622, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"templates/viz.html": templatesVizHtml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"templates": &bintree{nil, map[string]*bintree{
		"viz.html": &bintree{templatesVizHtml, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
