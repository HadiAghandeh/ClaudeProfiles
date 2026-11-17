//go:build gui && linux

package main

/*
#cgo pkg-config: gtk+-3.0
#include <stdlib.h>
#include <gtk/gtk.h>

static void set_icon(void *win, const char *path) {
	gtk_window_set_icon_from_file(GTK_WINDOW(win), path, NULL);
}

// pick_folder runs GTK's own folder chooser; the result must be g_free'd.
static char *pick_folder(void *parent, const char *start) {
	GtkWidget *d = gtk_file_chooser_dialog_new("Choose the project folder for Claude Code",
		GTK_WINDOW(parent), GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER,
		"_Cancel", GTK_RESPONSE_CANCEL, "_Select", GTK_RESPONSE_ACCEPT, NULL);
	gtk_file_chooser_set_create_folders(GTK_FILE_CHOOSER(d), TRUE);
	if (start && *start) gtk_file_chooser_set_current_folder(GTK_FILE_CHOOSER(d), start);
	char *res = NULL;
	if (gtk_dialog_run(GTK_DIALOG(d)) == GTK_RESPONSE_ACCEPT)
		res = gtk_file_chooser_get_filename(GTK_FILE_CHOOSER(d));
	gtk_widget_destroy(d);
	return res;
}

static void free_gstr(char *s) { g_free(s); }
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	webview "github.com/webview/webview_go"
)

// newWindow opens the WebKitGTK window. GTK scales it for HiDPI displays.
func newWindow() (view, error) {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return nil, errors.New("no graphical display found; use claude-profiles in a terminal instead")
	}
	w := webview.New(false)
	if w == nil || w.Window() == nil {
		return nil, errors.New("could not open the window; install WebKitGTK (libwebkit2gtk-4.1-0) or use claude-profiles in a terminal instead")
	}
	w.SetTitle("Claude Profiles")
	w.SetSize(560, 420, webview.HintMin)
	w.SetSize(780, 600, webview.HintNone)
	if icon := iconFile(); icon != "" {
		cs := C.CString(icon)
		C.set_icon(w.Window(), cs)
		C.free(unsafe.Pointer(cs))
	}
	return w, nil
}

// pickFolder shows GTK's folder chooser. GTK must be used from the UI thread,
// so it runs from the event loop, after the bound call has returned.
func pickFolder(w view, start string, done func(string)) {
	win, _ := w.(interface{ Window() unsafe.Pointer })
	w.Dispatch(func() {
		var parent unsafe.Pointer
		if win != nil {
			parent = win.Window()
		}
		cs := C.CString(start)
		defer C.free(unsafe.Pointer(cs))
		res := C.pick_folder(parent, cs)
		picked := ""
		if res != nil {
			picked = C.GoString(res)
			C.free_gstr(res)
		}
		done(picked)
	})
}

func openInFileManager(path string) error { return exec.Command("xdg-open", path).Start() }

// showError reports a problem when there may be no terminal to print to.
func showError(msg string) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	for _, c := range [][]string{
		{"zenity", "--error", "--title=Claude Profiles", "--text=" + msg},
		{"kdialog", "--title", "Claude Profiles", "--error", msg},
		{"notify-send", "Claude Profiles", msg},
	} {
		if p, err := exec.LookPath(c[0]); err == nil {
			_ = exec.Command(p, c[1:]...).Run()
			return
		}
	}
}
