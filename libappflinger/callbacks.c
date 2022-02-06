#include "callbacks.h"

int invoke_on_ui_video_frame(on_ui_video_frame_cb_t *cb, const char *session_id, int is_codec_config, int is_key_frame, int idx, long long pts,
    long long dts, void *data, unsigned data_len)
{
    return cb(session_id, is_codec_config, is_key_frame, idx, pts, dts, data, data_len);
}

int invoke_on_ui_image_frame(on_ui_image_frame_cb_t *cb, const char *session_id, int x, int y, int width, int height, int is_frame, 
    void *img_data, unsigned img_size, void *alpha_data, unsigned alpha_size)
{
    return cb(session_id, x, y, width, height, is_frame, img_data, img_size, alpha_data, alpha_size);
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