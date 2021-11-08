#ifndef __callbacks__

#define __callbacks__

#ifdef __cplusplus
extern "C" {
#endif


typedef int on_ui_video_frame_cb_t(const char *session_id, int is_codec_config, int is_key_frame, int idx, long long pts, long long dts,
    void *data, unsigned data_len);

typedef int on_ui_image_frame_cb_t(const char *session_id, int x, int y, int width, int height, int img_size, int alpha_size, 
    int is_frame, void *img_data, void *alpha_data);

typedef int load_cb_t(const char *session_id, const char *instance_id, const char *url);

typedef int cancel_load_cb_t(const char *session_id, const char *instance_id);

typedef int pause_cb_t(const char *session_id, const char *instance_id);

typedef int play_cb_t(const char *session_id, const char *instance_id);

typedef int seek_cb_t(const char *session_id, const char *instance_id, double time);

typedef int get_paused_cb_t(const char *session_id, const char *instance_id, int *paused);

typedef int get_seeking_cb_t(const char *session_id, const char *instance_id, int *seeking);

typedef int get_duration_cb_t(const char *session_id, const char *instance_id, double *duration);

typedef int get_current_time_cb_t(const char *session_id, const char *instance_id, double *current_time);

typedef int get_network_state_cb_t(const char *session_id, const char *instance_id, int *network_state);

typedef int get_ready_state_cb_t(const char *session_id, const char *instance_id, int *ready_state);

typedef int set_rect_cb_t(const char *session_id, const char *instance_id, int x, int y, int width , int height);

typedef struct appflinger_callbacks_struct
{
    on_ui_video_frame_cb_t *on_ui_video_frame_cb;
    on_ui_image_frame_cb_t *on_ui_image_frame_cb;
    load_cb_t *load_cb;
    set_rect_cb_t *set_rect_cb;
    cancel_load_cb_t *cancel_load_cb;
    pause_cb_t *pause_cb;
    play_cb_t *play_cb;
    seek_cb_t *seek_cb;
    get_paused_cb_t *get_paused_cb;
    get_seeking_cb_t *get_seeking_cb;
    get_duration_cb_t *get_duration_cb;
    get_current_time_cb_t *get_current_time_cb;
    get_network_state_cb_t *get_network_state_cb;
    get_ready_state_cb_t *get_ready_state_cb;
} appflinger_callbacks_t;

// Helper functions to invoke the above CBs from Go
int invoke_on_ui_video_frame(on_ui_video_frame_cb_t *cb, const char *session_id, int is_codec_config, int is_key_frame, int idx, long long pts,
    long long dts, void *data, unsigned data_len);

int invoke_on_ui_image_frame(on_ui_image_frame_cb_t *cb, const char *session_id, int x, int y, int width, int height, int img_size, int alpha_size, int is_frame, 
    void *img_data, void *alpha_data);

int invoke_load(load_cb_t *cb, const char *session_id, const char *instance_id, const char *url);

int invoke_cancel_load(cancel_load_cb_t *cb, const char *session_id, const char *instance_id);

int invoke_pause(pause_cb_t *cb, const char *session_id, const char *instance_id);

int invoke_play(play_cb_t *cb, const char *session_id, const char *instance_id);

int invoke_seek(seek_cb_t *cb, const char *session_id, const char *instance_id, double time);

int invoke_get_paused(get_paused_cb_t *cb, const char *session_id, const char *instance_id, int *paused);

int invoke_get_seeking(get_seeking_cb_t *cb, const char *session_id, const char *instance_id, int *seeking);

int invoke_get_duration(get_duration_cb_t *cb, const char *session_id, const char *instance_id, double *duration);

int invoke_get_current_time(get_current_time_cb_t *cb, const char *session_id, const char *instance_id, double *current_time);

int invoke_get_network_state(get_network_state_cb_t *cb, const char *session_id, const char *instance_id, int *network_state);

int invoke_get_ready_state(get_ready_state_cb_t *cb, const char *session_id, const char *instance_id, int *ready_state);

int invoke_set_rect(set_rect_cb_t *cb, const char *session_id, const char *instance_id, int x, int y, int width , int height);


#ifdef __cplusplus
}
#endif

#endif // ndef __callbacks__