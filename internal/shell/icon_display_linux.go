//go:build linux && cgo

package shell

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>

static void wpSetWindowIcon(void *win, const unsigned char *data, size_t n) {
	GdkPixbufLoader *loader = gdk_pixbuf_loader_new();
	if (loader == NULL) return;
	if (gdk_pixbuf_loader_write(loader, data, n, NULL) && gdk_pixbuf_loader_close(loader, NULL)) {
		GdkPixbuf *pixbuf = gdk_pixbuf_loader_get_pixbuf(loader);
		if (pixbuf != NULL) gtk_window_set_icon(GTK_WINDOW(win), pixbuf);
	}
	g_object_unref(loader);
}
*/
import "C"

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// setDisplayWindowIcon gives a display window the icon of the displays, not the one of the operator window. It runs on the main thread.
func setDisplayWindowIcon(w *application.WebviewWindow) {
	win := w.NativeWindow()
	if win == nil || len(displayIconPNG) == 0 {
		return
	}
	C.wpSetWindowIcon(win, (*C.uchar)(unsafe.Pointer(&displayIconPNG[0])), C.size_t(len(displayIconPNG)))
}
