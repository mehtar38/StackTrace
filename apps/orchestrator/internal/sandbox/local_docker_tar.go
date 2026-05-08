package sandbox

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// addDirToTarImpl recursively walks srcDir and writes every file into tw.
// Each entry's name is relative to srcDir, with prefix prepended.
// Hidden files (dot-prefixed) and node_modules are skipped.
func addDirToTarImpl(tw *tar.Writer, srcDir, prefix string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip node_modules — they're massive and not needed in build context
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}

		// Compute the tar entry name relative to srcDir
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		entryName := rel
		if prefix != "" {
			entryName = prefix + "/" + rel
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Write directory entry
		if d.IsDir() {
			hdr := &tar.Header{
				Name:     entryName + "/",
				Typeflag: tar.TypeDir,
				Mode:     int64(info.Mode()),
				ModTime:  info.ModTime(),
			}
			return tw.WriteHeader(hdr)
		}

		// Write file entry
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = entryName

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}
