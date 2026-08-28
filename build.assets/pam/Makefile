UNAME := $(shell uname -s)

# PAM_POLICY_PATH is where the PAM policy for PAM-aware applications is
# defined.
PAM_POLICY_PATH = /etc/pam.d/

PAM_MODULE_PATH_ESCAPED = $(shell pwd | sed 's/\//\\\//g')

# LD and LD_FLAGS controls the linker and linker flags to use and are
# determined by the OS.
LD = ld
LD_FLAGS = -lpam --shared -x
ifeq ($(UNAME),Darwin)
    LD = clang
    LD_FLAGS = -lpam -shared
endif

all: pam_teleport.so

install: pam_teleport.so
	# Copy all PAM policy files over to /etc/pam.d
	for file in $(shell ls policy/); do \
		# Replace the module path in these files with the absolute path to \
		# pam_teleport.so in current directory. That way we don't depend on the \
		# OS-specific PAM module locations. \
		sh -c "sed 's/pam_teleport.so/$(PAM_MODULE_PATH_ESCAPED)\/pam_teleport.so/' policy/$${file} >$(PAM_POLICY_PATH)$${file}"; \
	done

pam_teleport.so: pam_teleport.o
	$(LD) $(LD_FLAGS) -o pam_teleport.so pam_teleport.o
	chmod 644 pam_teleport.so

pam_teleport.o: clean pam_teleport.c
	gcc -fPIC -c pam_teleport.c

clean:
	rm -f pam_teleport.o pam_teleport.so
