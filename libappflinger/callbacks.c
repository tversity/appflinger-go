#include "callbacks.h"

int invoke_on_ui_frame(on_ui_frame_cb_t *cb, const char *session_id, int is_codec_config, int is_key_frame, int idx, long long pts,
    long long dts, void *data, unsigned data_len)
{
    return cb(session_id, is_codec_config, is_key_frame, idx, pts, dts, data, data_len);
}

int invoke_load(load_cb_t *cb, const char *session_id, const char *instance_id, const char *url)
{
    return cb(session_id, instance_id, url);
}

int invoke_cancel_load(cancel_load_cb_t *cb, const char *session_id, const char *instance_id)
{
    return cb(session_id, instance_id);
}

int invoke_pause(pause_cb_t *cb, const char *session_id, const char *instance_id)
{
    return cb(session_id, instance_id);
}

int invoke_play(play_cb_t *cb, const char *session_id, const char *instance_id)
{
    return cb(session_id, instance_id);
}

int invoke_seek(seek_cb_t *cb, const char *session_id, const char *instance_id, double time)
{
    return cb(session_id, instance_id, time);
}

int invoke_get_paused(get_paused_cb_t *cb, const char *session_id, const char *instance_id, int *paused)
{
    return cb(session_id, instance_id, paused);
}

int invoke_get_seeking(get_seeking_cb_t *cb, const char *session_id, const char *instance_id, int *seeking)
{
    return cb(session_id, instance_id, seeking);
}

int invoke_get_duration(get_duration_cb_t *cb, const char *session_id, const char *instance_id, double *duration)
{
    return cb(session_id, instance_id, duration);
}

int invoke_get_current_time(get_current_time_cb_t *cb, const char *session_id, const char *instance_id, double *current_time)
{
    return cb(session_id, instance_id, current_time);
}

int invoke_get_network_state(get_network_state_cb_t *cb, const char *session_id, const char *instance_id, int *network_state)
{
    return cb(session_id, instance_id, network_state);
}

int invoke_get_ready_state(get_ready_state_cb_t *cb, const char *session_id, const char *instance_id, int *ready_state)
{
    return cb(session_id, instance_id, ready_state);
}

int invoke_set_rect(set_rect_cb_t *cb, const char *session_id, const char *instance_id, int x, int y, int width , int height)
{
    return cb(session_id, instance_id, x, y, width, height);
}

int invoke_add_source_buffer(add_source_buffer_cb_t *cb, const char *session_id, const char *instance_id, const char *source_id, const char *mime_type)
{
    return cb(session_id, instance_id, source_id, mime_type);
}

int invoke_remove_source_buffer(remove_source_buffer_cb_t *cb, const char *session_id, const char *instance_id, const char *source_id)
{
    return cb(session_id, instance_id, source_id);
}

int invoke_abort_source_buffer(abort_source_buffer_cb_t *cb, const char *session_id, const char *instance_id, const char *source_id)
{
    return cb(session_id, instance_id, source_id);
}

int invoke_append_buffer(append_buffer_cb_t *cb, const char *session_id, const char *instance_id, const char *source_id, double append_window_start, double append_window_end,
    const char *buffer_id, int buffer_offset, int buffer_length, void *payload, unsigned payload_length, void *buffered_start, void *buffered_end, int *buffered_length)
{
    return cb(session_id, instance_id, source_id, append_window_start, append_window_end, buffer_id, buffer_offset, buffer_length, payload, payload_length, buffered_start, buffered_end, buffered_length);
}

int invoke_set_append_mode(set_append_mode_cb_t *cb, const char *session_id, const char *instance_id, const char *source_id, int mode)
{
    return cb(session_id, instance_id, source_id, mode);
}

int invoke_set_append_timestamp_offset(set_append_timestamp_offset_cb_t *cb, const char *session_id, const char *instance_id, const char *source_id, double timestamp_offset)
{
    return cb(session_id, instance_id, source_id, timestamp_offset);
}

int invoke_remove_buffer_range(remove_buffer_range_cb_t *cb, const char *session_id, const char *instance_id, const char *source_id, double start, double end)
{
    return cb(session_id, instance_id, source_id, start, end);
}

int invoke_change_source_buffer_type(change_source_buffer_type_cb_t *cb, const char *session_id, const char *instance_id, const char *source_id, const char *mime_type)
{
    return cb(session_id, instance_id, source_id, mime_type);
}
