#include <fcntl.h>
#include <string.h>
#include <unistd.h>

int main(int argc, char **argv) {
    if (argc != 2) {
        return 1;
    }

    const char *successStr = "success";
    if (strncmp(argv[1], successStr, sizeof(successStr)) == 0) {
        if (!fork()) {
            const char *const args[] = {
                "can you see",
                "me?",
                NULL,
            };

            open("/etc/hostname", O_RDONLY);
            execv("/usr/bin/echo", (char *const *)args);
        }

        return 0;
    } else {
        if (!fork()) {
            const char *const args[] = {
                "this should",
                "fail.",
                NULL,
            };

            open("/who/now", O_RDONLY);
            execv("whereami", (char *const *)args);
        }

        return 42;
    }
}
