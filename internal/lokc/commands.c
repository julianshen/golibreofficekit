#include "commands.h"
#include "LibreOfficeKit/LibreOfficeKit.h"
#include <stdlib.h>
#include <string.h>

int loke_get_command_values(void* doc, const char* command, char** out, size_t* out_len) {
    if (!doc || !command || !out || !out_len) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->getCommandValues) return 0;
    char* s = d->pClass->getCommandValues(d, command);
    if (!s) return 0;
    *out_len = strlen(s);
    *out = s;
    return 1;
}

int loke_complete_function(void* doc, const char* name) {
    if (!doc || !name) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->completeFunction) return 0;
    d->pClass->completeFunction(d, name);
    return 1;
}

int loke_doc_send_dialog_event(void* doc, uint64_t window_id, const char* args_json) {
    if (!doc || !args_json) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->sendDialogEvent) return 0;
    d->pClass->sendDialogEvent(d, window_id, args_json);
    return 1;
}

int loke_doc_send_content_control_event(void* doc, const char* args_json) {
    if (!doc || !args_json) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->sendContentControlEvent) return 0;
    d->pClass->sendContentControlEvent(d, args_json);
    return 1;
}

int loke_doc_send_form_field_event(void* doc, const char* args_json) {
    if (!doc || !args_json) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->sendFormFieldEvent) return 0;
    d->pClass->sendFormFieldEvent(d, args_json);
    return 1;
}
