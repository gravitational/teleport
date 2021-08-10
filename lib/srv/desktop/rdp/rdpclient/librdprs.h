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

typedef struct CGOBitmap {
  uint16_t dest_left;
  uint16_t dest_top;
  uint16_t dest_right;
  uint16_t dest_bottom;
  uint8_t *data_ptr;
  uintptr_t data_len;
  uintptr_t data_cap;
} CGOBitmap;

char *connect_rdp(const char *go_addr,
                  const char *go_username,
                  const char *go_password,
                  uint16_t screen_width,
                  uint16_t screen_height,
                  int64_t client_ref);

char *read_rdp_output(int64_t client_ref);

char *write_rdp_pointer(int64_t client_ref, struct Pointer pointer);

char *write_rdp_keyboard(int64_t client_ref, struct Key key);

char *close_rdp(int64_t client_ref);

void free_rust_string(char *s);

extern void free_go_string(char *s);

extern char *handle_bitmap(int64_t client_ref, struct CGOBitmap b);
