# CMake toolchain used to build grpc_node_plugin for arm64 which is needed by grpc-tools.
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -march=armv8-a" CACHE STRING "c++ flags")
set(CMAKE_C_FLAGS   "${CMAKE_C_FLAGS} -march=armv8-a" CACHE STRING "c flags")
set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} -march=armv8-a" CACHE STRING "ld flags")
