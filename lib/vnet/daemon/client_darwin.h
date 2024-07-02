#ifndef client_darwin_h
#define client_darwin_h

#import <Foundation/Foundation.h>
#import "protocol_darwin.h"
#import "common_darwin.h"

typedef struct BundlePathResult {
    const char * bundlePath;
} BundlePathResult;

// BundlePath updates bundlePath of result with the path
// to the bundle directory that contains the current executable.
// bundlePath is an empty string if the bundle details could not be fetched.
// It might return a path even for executables that are not in a bundle.
// In that case, calling codesign --verify on that path will simply return with 1.
void BundlePath(struct BundlePathResult *result);

// DaemonPlist takes the result of DaemonLabel and appends ".plist" to it
// if not empty.
NSString * DaemonPlist(void);

typedef struct RegisterDaemonResult {
    bool ok;
    // service_status is fetched even if [service registerAndReturnError] fails,
    // for debugging purposes.
    int service_status;
    const char * error_description;
} RegisterDaemonResult;

// RegisterDaemon attempts to register the daemon. After the registration attempt,
// it fetches the daemon status.
// Pretty much a noop if the daemon is already registered and enabled.
void RegisterDaemon(struct RegisterDaemonResult *result);

// DaemonStatus returns the current status of the daemon's service in SMAppService.
// Returns -1 if the given macOS version doesn't support SMAppService.
int DaemonStatus(void);

void OpenSystemSettingsLoginItems(void);

typedef struct StartVnetRequest {
    struct VnetConfig * vnet_config;
} StartVnetRequest;

typedef struct StartVnetResponse {
    bool ok;
    const char * error_domain;
    const char * error_description;
} StartVnetResponse;

// StartVnet spawns the daemon process. Only the first call does that,
// subsequent calls are noops. The daemon process exits after the socket file
// in request.vnet_config.socket_path is removed. After that it can be spawned
// again by calling StartVnet.
//
// Blocks until the daemon receives the message or until the client gets
// invalidated.
//
// After calling StartVnet, the caller is expected to call InvalidateDaemonClient
// when a surrounding context in Go gets canceled.
void StartVnet(struct StartVnetRequest *request, struct StartVnetResponse *response);

// InvalidateDaemonClient closes the connection to the daemon and unblocks
// any calls awaiting a reply from the daemon.
void InvalidateDaemonClient(void);

@interface VNEDaemonClient : NSObject
-(void)startVnet:(VnetConfig *)vnetConfig completion:(void (^)(NSError * error))completion;
// invalidate executes all outstanding reply blocks, error handling blocks,
// and invalidation blocks and forbids from sending or receiving new messages.
-(void)invalidate;
@end

#endif /* client_darwin_h */
