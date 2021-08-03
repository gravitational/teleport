#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

typedef enum CGOPointerButton {
  PointerButtonNone,
  PointerButtonLeft,
  PointerButtonRight,
  PointerButtonMiddle,
} CGOPointerButton;

typedef struct CGOString {
  uint8_t *data;
  uint16_t len;
} CGOString;

typedef struct CGOString CGOError;

typedef struct CGOBitmap {
  uint16_t dest_left;
  uint16_t dest_top;
  uint16_t dest_right;
  uint16_t dest_bottom;
  const uint8_t *data_ptr;
  uintptr_t data_len;
} CGOBitmap;

typedef struct Pointer {
  uint16_t x;
  uint16_t y;
  enum CGOPointerButton button;
  bool down;
} Pointer;

typedef struct Key {
  uint16_t code;
  bool down;
} Key;

CGOError connect_rdp(struct CGOString go_addr,
                     struct CGOString go_username,
                     struct CGOString go_password,
                     uint16_t screen_width,
                     uint16_t screen_height,
                     int64_t client_ref);

CGOError read_rdp_output(int64_t client_ref, CGOError (*handle_bitmap)(int64_t, struct CGOBitmap));

CGOError write_rdp_pointer(int64_t client_ref, struct Pointer pointer);

CGOError write_rdp_keyboard(int64_t client_ref, struct Key key);

CGOError close_rdp(int64_t client_ref);
