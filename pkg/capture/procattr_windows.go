//go:build windows

package capture

import "syscall"

// sysProcAttrHidden configura la ejecución de procesos secundarios de manera 100% invisible
// sin asignación de consola ni parpadeo de ventanas (CREATE_NO_WINDOW = 0x08000000).
func sysProcAttrHidden() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

