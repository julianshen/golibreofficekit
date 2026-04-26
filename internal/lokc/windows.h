#ifndef LOKC_WINDOWS_H
#define LOKC_WINDOWS_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Window events
int loke_post_window_key_event(void* doc, uint32_t window_id, int type, int char_code, int key_code);
int loke_post_window_mouse_event(void* doc, uint32_t window_id, int type, int64_t x, int64_t y, int count, int buttons, int mods);
int loke_post_window_gesture_event(void* doc, uint32_t window_id, const char* typ, int64_t x, int64_t y, int64_t offset);
int loke_post_window_ext_text_input_event(void* doc, uint32_t window_id, int typ, const char* text);
int loke_resize_window(void* doc, uint32_t window_id, int w, int h);

// Window paint — x, y are top-left of source rect in twips
int loke_paint_window(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h);
int loke_paint_window_dpi(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h, double dpiscale);
int loke_paint_window_for_view(void* doc, uint32_t window_id, int view_id, void* buf, int x, int y, int px_w, int px_h, double dpiscale);

#ifdef __cplusplus
}
#endif

#endif
