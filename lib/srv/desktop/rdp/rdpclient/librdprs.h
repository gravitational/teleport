#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

#define NTSTATUS_OK 0

#define SPECIAL_NO_RESPONSE 4294967295

typedef enum CGOPointerButton {
  PointerButtonNone,
  PointerButtonLeft,
  PointerButtonRight,
  PointerButtonMiddle,
} CGOPointerButton;

typedef enum CGOPointerWheel {
  PointerWheelNone,
  PointerWheelVertical,
  PointerWheelHorizontal,
} CGOPointerWheel;

typedef struct Client Client;

typedef char *CGOError;

typedef struct ClientOrError {
  struct Client *client;
  CGOError err;
} ClientOrError;

typedef struct CGOMousePointerEvent {
  uint16_t x;
  uint16_t y;
  enum CGOPointerButton button;
  bool down;
  enum CGOPointerWheel wheel;
  int16_t wheel_delta;
} CGOMousePointerEvent;

typedef struct CGOKeyboardEvent {
  uint16_t code;
  bool down;
} CGOKeyboardEvent;

typedef struct CGOBitmap {
  uint16_t dest_left;
  uint16_t dest_top;
  uint16_t dest_right;
  uint16_t dest_bottom;
  uint8_t *data_ptr;
  uintptr_t data_len;
  uintptr_t data_cap;
} CGOBitmap;

void init(void);

struct ClientOrError connect_rdp(char *go_addr,
                                 char *go_username,
                                 uint32_t cert_der_len,
                                 uint8_t *cert_der,
                                 uint32_t key_der_len,
                                 uint8_t *key_der,
                                 uint16_t screen_width,
                                 uint16_t screen_height);

CGOError read_rdp_output(struct Client *client_ptr, uintptr_t client_ref);

CGOError write_rdp_pointer(struct Client *client_ptr, struct CGOMousePointerEvent pointer);

CGOError write_rdp_keyboard(struct Client *client_ptr, struct CGOKeyboardEvent key);

CGOError close_rdp(struct Client *client_ptr);

void free_rdp(struct Client *client_ptr);

void free_rust_string(char *s);

extern void free_go_string(char *s);

extern CGOError handle_bitmap(uintptr_t client_ref, struct CGOBitmap b);
