// Copyright 2020 Matthew Holt

package diskspace

import (
	"runtime"

	syscall "golang.org/x/sys/unix"
)

type diskStatus struct {
	all, available, free, used uint64
}

// Source: https://gist.github.com/ttys3/21e2a1215cf1905ab19ddcec03927c75
func diskUsage(path string) (diskStatus, error) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return diskStatus{}, err
	}
	disk := diskStatus{
		all:       fs.Blocks * uint64(fs.Bsize),
		available: fs.Bavail * uint64(fs.Bsize),
		free:      fs.Bfree * uint64(fs.Bsize),
	}
	if runtime.GOOS == "darwin" {
		// not sure why mac is different but whatevs
		disk.used = disk.all - disk.available
	} else {
		disk.used = disk.all - disk.free
	}
	return disk, nil
}
