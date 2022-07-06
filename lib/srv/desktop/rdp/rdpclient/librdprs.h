#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

#define NTSTATUS_OK 0

#define SPECIAL_NO_RESPONSE 4294967295

#define SCARD_DEVICE_ID 1

#define VERSION_MAJOR 1

#define VERSION_MINOR 12

#define SMARTCARD_CAPABILITY_VERSION_01 1

#define DRIVE_CAPABILITY_VERSION_02 2

#define GENERAL_CAPABILITY_VERSION_01 1

#define GENERAL_CAPABILITY_VERSION_02 2

/**
 * The default maximum chunk size for virtual channel data.
 *
 * If an RDP server supports larger chunks, it will advertise
 * the larger chunk size in the `VCChunkSize` field of the
 * virtual channel capability set.
 *
 * See also:
 * - https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/6c074267-1b32-4ceb-9496-2eb941a23e6b
 * - https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/a8593178-80c0-4b80-876c-cb77e62cecfc
 */
#define CHANNEL_CHUNK_LEGNTH 1600

typedef enum CGOErrCode {
  ErrCodeSuccess = 0,
  ErrCodeFailure = 1,
} CGOErrCode;

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

typedef enum FileType {
  File = 0,
  Directory = 1,
} FileType;

typedef enum TdpErrCode {
  /**
   * nil (no error, operation succeeded)
   */
  Nil = 0,
  /**
   * operation failed
   */
  Failed = 1,
  /**
   * resource does not exist
   */
  DoesNotExist = 2,
  /**
   * resource already exists
   */
  AlreadyExists = 3,
} TdpErrCode;

/**
 * Client has an unusual lifecycle:
 * - connect_rdp creates it on the heap, grabs a raw pointer and returns in to Go
 * - most other exported rdp functions take the raw pointer, convert it to a reference for use
 *   without dropping the Client
 * - free_rdp takes the raw pointer and drops it
 *
 * All of the exported rdp functions could run concurrently, so the rdp_client is synchronized.
 * tcp_fd is only set in connect_rdp and used as read-only afterwards, so it does not need
 * synchronization.
 */
typedef struct Client Client;

typedef struct ClientOrError {
  struct Client *client;
  enum CGOErrCode err;
} ClientOrError;

typedef struct CGOSharedDirectoryAnnounce {
  uint32_t directory_id;
  const char *name;
} CGOSharedDirectoryAnnounce;

typedef struct CGOFileSystemObject {
  uint64_t last_modified;
  uint64_t size;
  enum FileType file_type;
  const char *path;
} CGOFileSystemObject;

typedef struct CGOSharedDirectoryInfoResponse {
  uint32_t completion_id;
  enum TdpErrCode err_code;
  struct CGOFileSystemObject fso;
} CGOSharedDirectoryInfoResponse;

/**
 * SharedDirectoryCreateResponse is sent by the TDP client to the server
 * to acknowledge a SharedDirectoryCreateRequest was received and executed.
 */
typedef struct SharedDirectoryCreateResponse {
  uint32_t completion_id;
  enum TdpErrCode err_code;
} SharedDirectoryCreateResponse;

typedef struct SharedDirectoryCreateResponse CGOSharedDirectoryCreateResponse;

typedef struct SharedDirectoryCreateResponse CGOSharedDirectoryDeleteResponse;

typedef struct CGOSharedDirectoryListResponse {
  uint32_t completion_id;
  enum TdpErrCode err_code;
  uint32_t fso_list_length;
  struct CGOFileSystemObject *fso_list;
} CGOSharedDirectoryListResponse;

typedef struct CGOSharedDirectoryReadResponse {
  uint32_t completion_id;
  enum TdpErrCode err_code;
  uint32_t read_data_length;
  uint8_t *read_data;
} CGOSharedDirectoryReadResponse;

typedef struct SharedDirectoryWriteResponse {
  uint32_t completion_id;
  enum TdpErrCode err_code;
  uint32_t bytes_written;
} SharedDirectoryWriteResponse;

typedef struct SharedDirectoryWriteResponse CGOSharedDirectoryWriteResponse;

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

/**
 * SharedDirectoryAcknowledge is sent by the TDP server to the client
 * to acknowledge that a SharedDirectoryAnnounce was received.
 */
typedef struct SharedDirectoryAcknowledge {
  enum TdpErrCode err_code;
  uint32_t directory_id;
} SharedDirectoryAcknowledge;

typedef struct SharedDirectoryAcknowledge CGOSharedDirectoryAcknowledge;

typedef struct CGOSharedDirectoryInfoRequest {
  uint32_t completion_id;
  uint32_t directory_id;
  const char *path;
} CGOSharedDirectoryInfoRequest;

typedef struct CGOSharedDirectoryCreateRequest {
  uint32_t completion_id;
  uint32_t directory_id;
  enum FileType file_type;
  const char *path;
} CGOSharedDirectoryCreateRequest;

typedef struct CGOSharedDirectoryInfoRequest CGOSharedDirectoryDeleteRequest;

typedef struct CGOSharedDirectoryInfoRequest CGOSharedDirectoryListRequest;

typedef struct CGOSharedDirectoryReadRequest {
  uint32_t completion_id;
  uint32_t directory_id;
  uint32_t path_length;
  const char *path;
  uint64_t offset;
  uint32_t length;
} CGOSharedDirectoryReadRequest;

typedef struct CGOSharedDirectoryWriteRequest {
  uint32_t completion_id;
  uint32_t directory_id;
  uint64_t offset;
  uint32_t path_length;
  const char *path;
  uint32_t write_data_length;
  uint8_t *write_data;
} CGOSharedDirectoryWriteRequest;













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
                                 const char *go_addr,
                                 const char *go_username,
                                 uint32_t cert_der_len,
                                 uint8_t *cert_der,
                                 uint32_t key_der_len,
                                 uint8_t *key_der,
                                 uint16_t screen_width,
                                 uint16_t screen_height,
                                 bool allow_clipboard,
                                 bool allow_directory_sharing);

/**
 * `update_clipboard` is called from Go, and caches data that was copied
 * client-side while notifying the RDP server that new clipboard data is available.
 *
 * # Safety
 *
 * client_ptr MUST be a valid pointer.
 * (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
 *
 * data MUST be a valid pointer.
 * (validity defined by the validity of data in https://doc.rust-lang.org/std/slice/fn.from_raw_parts_mut.html)
 */
enum CGOErrCode update_clipboard(struct Client *client_ptr,
                                 uint8_t *data,
                                 uint32_t len);

/**
 * handle_tdp_sd_announce announces a new drive that's ready to be
 * redirected over RDP.
 *
 *
 * # Safety
 *
 * client_ptr MUST be a valid pointer.
 * (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
 *
 * sd_announce.name MUST be a non-null pointer to a C-style null terminated string.
 */
enum CGOErrCode handle_tdp_sd_announce(struct Client *client_ptr,
                                       struct CGOSharedDirectoryAnnounce sd_announce);

/**
 * handle_tdp_sd_info_response handles a TDP Shared Directory Info Response
 * message
 *
 * # Safety
 *
 * client_ptr MUST be a valid pointer.
 * (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
 *
 * res.fso.path MUST be a non-null pointer to a C-style null terminated string.
 */
enum CGOErrCode handle_tdp_sd_info_response(struct Client *client_ptr,
                                            struct CGOSharedDirectoryInfoResponse res);

/**
 * handle_tdp_sd_create_response handles a TDP Shared Directory Create Response
 * message
 *
 * # Safety
 *
 * client_ptr MUST be a valid pointer.
 * (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
 */
enum CGOErrCode handle_tdp_sd_create_response(struct Client *client_ptr,
                                              CGOSharedDirectoryCreateResponse res);

/**
 * handle_tdp_sd_delete_response handles a TDP Shared Directory Delete Response
 * message
 *
 * # Safety
 *
 * client_ptr MUST be a valid pointer.
 * (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
 */
enum CGOErrCode handle_tdp_sd_delete_response(struct Client *client_ptr,
                                              CGOSharedDirectoryDeleteResponse res);

/**
 * handle_tdp_sd_list_response handles a TDP Shared Directory List Response message.
 *
 * # Safety
 *
 * client_ptr MUST be a valid pointer.
 * (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
 *
 * res.fso_list MUST be a valid pointer
 * (validity defined by the validity of data in https://doc.rust-lang.org/std/slice/fn.from_raw_parts_mut.html)
 *
 * each res.fso_list[i].path MUST be a non-null pointer to a C-style null terminated string.
 */
enum CGOErrCode handle_tdp_sd_list_response(struct Client *client_ptr,
                                            struct CGOSharedDirectoryListResponse res);

/**
 * handle_tdp_sd_read_response handles a TDP Shared Directory Read Response
 * message
 *
 * # Safety
 *
 * client_ptr must be a valid pointer
 */
enum CGOErrCode handle_tdp_sd_read_response(struct Client *client_ptr,
                                            struct CGOSharedDirectoryReadResponse res);

/**
 * handle_tdp_sd_write_response handles a TDP Shared Directory Write Response
 * message
 *
 * # Safety
 *
 * client_ptr must be a valid pointer
 */
enum CGOErrCode handle_tdp_sd_write_response(struct Client *client_ptr,
                                             CGOSharedDirectoryWriteResponse res);

/**
 * `read_rdp_output` reads incoming RDP bitmap frames from client at client_ref and forwards them to
 * handle_bitmap.
 *
 * # Safety
 *
 * `client_ptr` must be a valid pointer to a Client.
 * `handle_bitmap` *must not* free the memory of CGOBitmap.
 */
enum CGOErrCode read_rdp_output(struct Client *client_ptr);

/**
 * # Safety
 *
 * client_ptr MUST be a valid pointer.
 * (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
 */
enum CGOErrCode write_rdp_pointer(struct Client *client_ptr,
                                  struct CGOMousePointerEvent pointer);

/**
 * # Safety
 *
 * client_ptr MUST be a valid pointer.
 * (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
 */
enum CGOErrCode write_rdp_keyboard(struct Client *client_ptr,
                                   struct CGOKeyboardEvent key);

/**
 * # Safety
 *
 * client_ptr must be a valid pointer to a Client.
 */
enum CGOErrCode close_rdp(struct Client *client_ptr);

/**
 * free_rdp lets the Go side inform us when it's done with Client and it can be dropped.
 *
 * # Safety
 *
 * client_ptr MUST be a valid pointer.
 * (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
 */
void free_rdp(struct Client *client_ptr);

extern enum CGOErrCode handle_bitmap(uintptr_t client_ref, struct CGOBitmap *b);

extern enum CGOErrCode handle_remote_copy(uintptr_t client_ref, uint8_t *data, uint32_t len);

extern enum CGOErrCode tdp_sd_acknowledge(uintptr_t client_ref, CGOSharedDirectoryAcknowledge *ack);

extern enum CGOErrCode tdp_sd_info_request(uintptr_t client_ref,
                                           struct CGOSharedDirectoryInfoRequest *req);

extern enum CGOErrCode tdp_sd_create_request(uintptr_t client_ref,
                                             struct CGOSharedDirectoryCreateRequest *req);

extern enum CGOErrCode tdp_sd_delete_request(uintptr_t client_ref,
                                             CGOSharedDirectoryDeleteRequest *req);

extern enum CGOErrCode tdp_sd_list_request(uintptr_t client_ref,
                                           CGOSharedDirectoryListRequest *req);

extern enum CGOErrCode tdp_sd_read_request(uintptr_t client_ref,
                                           struct CGOSharedDirectoryReadRequest *req);

extern enum CGOErrCode tdp_sd_write_request(uintptr_t client_ref,
                                            struct CGOSharedDirectoryWriteRequest *req);
