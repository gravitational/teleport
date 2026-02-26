
define gha_build_profile_windows_all
  -F 'artifacts.windows[]=all'
endef

define gha_build_profile_cloud_only
  -F 'artifacts.linux.arch=amd64'
  -F 'artifacts.helm={}'
endef
