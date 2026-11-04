//go:build !windows

package capture

import "syscall"

func sysProcAttrHidden() *syscall.SysProcAttr {
	return nil
}

