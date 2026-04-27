#ifndef LOKC_COMMANDS_H
#define LOKC_COMMANDS_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// loke_get_command_values returns:
//   -1 → vtable slot missing / invalid arg
//    0 → LO accepted the call but returned NULL (no value)
//    1 → success; *out points at LOK-allocated buffer the caller frees
// On success, *out_len is the byte length (no null terminator).
int loke_get_command_values(void* doc, const char* command, char** out, size_t* out_len);

// Returns 1 on success, 0 on failure.
int loke_complete_function(void* doc, const char* name);

// Document-level sendDialogEvent (takes uint64 window ID)
int loke_doc_send_dialog_event(void* doc, uint64_t window_id, const char* args_json);

// Document-level sendContentControlEvent
int loke_doc_send_content_control_event(void* doc, const char* args_json);

// Document-level sendFormFieldEvent
int loke_doc_send_form_field_event(void* doc, const char* args_json);

#ifdef __cplusplus
}
#endif

#endif
