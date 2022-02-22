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

/**
 * Client has an unusual lifecycle:
 * - connect_rdp creates it on the heap, grabs a raw pointer and returns in to Go
 * - most other exported rdp functions take the raw pointer, convert it to a reference for use
 *   without dropping the Client
 * - close_rdp takes the raw pointer and drops it
 *
 * All of the exported rdp functions could run concurrently, so the rdp_client is synchronized.
 * tcp_fd is only set in connect_rdp and used as read-only afterwards, so it does not need
 * synchronization.
 */
typedef struct Client Client;

/**
 * CGOError is an alias for a C string pointer, for C API clarity.
 */
typedef char *CGOError;

typedef struct ClientOrError {
  struct Client *client;
  CGOError err;
} ClientOrError;

/**
 * CGOMousePointerEvent is a CGO-compatible version of PointerEvent that we pass back to Go.
 * PointerEvent is a mouse move or click update from the user.
 */
typedef struct CGOMousePointerEvent {
  uint16_t x;
  uint16_t y;
  enum CGOPointerButton button;
  bool down;
  enum CGOPointerWheel wheel;
  int16_t wheel_delta;
} CGOMousePointerEvent;

/**
 * CGOKeyboardEvent is a CGO-compatible version of KeyboardEvent that we pass back to Go.
 * KeyboardEvent is a keyboard update from the user.
 */
typedef struct CGOKeyboardEvent {
  uint16_t code;
  bool down;
} CGOKeyboardEvent;

/**
 * CGOBitmap is a CGO-compatible version of BitmapEvent that we pass back to Go.
 * BitmapEvent is a video output update from the server.
 */
typedef struct CGOBitmap {
  uint16_t dest_left;
  uint16_t dest_top;
  uint16_t dest_right;
  uint16_t dest_bottom;
  /**
   * The memory of this field is managed by the Rust side.
   */
  uint8_t *data_ptr;
  uintptr_t data_len;
  uintptr_t data_cap;
} CGOBitmap;

void init(void);

/**
 * connect_rdp establishes an RDP connection to go_addr with the provided credentials and screen
 * size. If succeeded, the client is internally registered under client_ref. When done with the
 * connection, the caller must call close_rdp.
 *
 * # Safety
 *
 * The caller mmust ensure that go_addr, go_username, cert_der, key_der point to valid buffers in respect
 * to their corresponding parameters.
 */
struct ClientOrError connect_rdp(uintptr_t go_ref,
                                 char *go_addr,
                                 char *go_username,
                                 uint32_t cert_der_len,
                                 uint8_t *cert_der,
                                 uint32_t key_der_len,
                                 uint8_t *key_der,
                                 uint16_t screen_width,
                                 uint16_t screen_height,
                                 bool allow_clipboard);

/**
 * `update_clipboard` is called from Go, and caches data that was copied
 * client-side while notifying the RDP server that new clipboard data is available.
 *
 * # Safety
 *
 * `client_ptr` must be a valid pointer to a Client.
 */
CGOError update_clipboard(struct Client *client_ptr, uint8_t *data, uint32_t len);

/**
 * `read_rdp_output` reads incoming RDP bitmap frames from client at client_ref and forwards them to
 * handle_bitmap.
 *
 * # Safety
 *
 * `client_ptr` must be a valid pointer to a Client.
 * `handle_bitmap` *must not* free the memory of CGOBitmap.
 */
CGOError read_rdp_output(struct Client *client_ptr);

/**
 * # Safety
 *
 * client_ptr must be a valid pointer to a Client.
 */
CGOError write_rdp_pointer(struct Client *client_ptr, struct CGOMousePointerEvent pointer);

/**
 * # Safety
 *
 * client_ptr must be a valid pointer to a Client.
 */
CGOError write_rdp_keyboard(struct Client *client_ptr, struct CGOKeyboardEvent key);

/**
 * # Safety
 *
 * client_ptr must be a valid pointer to a Client.
 */
CGOError close_rdp(struct Client *client_ptr);

/**
 * free_rdp lets the Go side inform us when it's done with Client and it can be dropped.
 *
 * # Safety
 *
 * client_ptr must be a valid pointer to a Client.
 */
void free_rdp(struct Client *client_ptr);

/**
 * # Safety
 *
 * The passed pointer must point to a C-style string allocated by Rust.
 */
void free_rust_string(char *s);

extern void free_go_string(char *s);

extern CGOError handle_bitmap(uintptr_t client_ref, struct CGOBitmap b);

extern CGOError handle_remote_copy(uintptr_t client_ref, uint8_t *data, uint32_t len);
