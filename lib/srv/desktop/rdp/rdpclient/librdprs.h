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

typedef struct Client Client;

typedef struct ClientOrError {
  struct Client *client;
  char *err;
} ClientOrError;

typedef struct CGOPointer {
  uint16_t x;
  uint16_t y;
  enum CGOPointerButton button;
  bool down;
} CGOPointer;

typedef struct CGOKey {
  uint16_t code;
  bool down;
} CGOKey;

typedef struct CGOBitmap {
  uint16_t dest_left;
  uint16_t dest_top;
  uint16_t dest_right;
  uint16_t dest_bottom;
  uint8_t *data_ptr;
  uintptr_t data_len;
  uintptr_t data_cap;
} CGOBitmap;

struct ClientOrError connect_rdp(const char *go_addr,
                                 const char *go_username,
                                 const char *go_password,
                                 uint16_t screen_width,
                                 uint16_t screen_height);

char *read_rdp_output(struct Client *client_ptr, int64_t client_ref);

char *write_rdp_pointer(struct Client *client_ptr, struct CGOPointer pointer);

char *write_rdp_keyboard(struct Client *client_ptr, struct CGOKey key);

char *close_rdp(struct Client *client_ptr);

void free_rust_string(char *s);

extern void free_go_string(char *s);

extern char *handle_bitmap(int64_t client_ref, struct CGOBitmap b);
