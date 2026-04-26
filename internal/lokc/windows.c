#include "windows.h"
#include "LibreOfficeKit/LibreOfficeKit.h"
#include <stdlib.h>
#include <string.h>

int loke_post_window_key_event(void* doc, uint32_t window_id, int type, int char_code, int key_code) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowKeyEvent) return 0;
    d->pClass->postWindowKeyEvent(d, window_id, type, char_code, key_code);
    return 1;
}

int loke_post_window_mouse_event(void* doc, uint32_t window_id, int type, int64_t x, int64_t y, int count, int buttons, int mods) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowMouseEvent) return 0;
    d->pClass->postWindowMouseEvent(d, window_id, type, x, y, count, buttons, mods);
    return 1;
}

int loke_post_window_gesture_event(void* doc, uint32_t window_id, const char* typ, int64_t x, int64_t y, int64_t offset) {
    if (!doc || !typ) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowGestureEvent) return 0;
    d->pClass->postWindowGestureEvent(d, window_id, typ, x, y, offset);
    return 1;
}

int loke_post_window_ext_text_input_event(void* doc, uint32_t window_id, int typ, const char* text) {
    if (!doc || !text) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowExtTextInputEvent) return 0;
    d->pClass->postWindowExtTextInputEvent(d, window_id, typ, text);
    return 1;
}

int loke_resize_window(void* doc, uint32_t window_id, int w, int h) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->resizeWindow) return 0;
    d->pClass->resizeWindow(d, window_id, w, h);
    return 1;
}

int loke_paint_window(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h) {
    if (!doc || !buf) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->paintWindow) return 0;
    d->pClass->paintWindow(d, window_id, (unsigned char*)buf, x, y, px_w, px_h);
    return 1;
}

int loke_paint_window_dpi(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h, double dpi_scale) {
    if (!doc || !buf) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->paintWindowDPI) return 0;
    d->pClass->paintWindowDPI(d, window_id, (unsigned char*)buf, x, y, px_w, px_h, dpi_scale);
    return 1;
}

int loke_paint_window_for_view(void* doc, uint32_t window_id, int view_id, void* buf, int x, int y, int px_w, int px_h, double dpi_scale) {
    if (!doc || !buf) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->paintWindowForView) return 0;
    d->pClass->paintWindowForView(d, window_id, (unsigned char*)buf, x, y, px_w, px_h, dpi_scale, view_id);
    return 1;
}
