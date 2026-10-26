# pf-serve: privacy-filter.cpp (PF_SRC, patched by build.sh) and the
# process magpie talks to.
cmake_minimum_required(VERSION 3.21)
project(pf-serve C CXX)

set(PF_SRC "" CACHE PATH "a privacy-filter.cpp checkout")
if (NOT PF_SRC)
    message(FATAL_ERROR "set PF_SRC, or run build.sh")
endif()
set(PF_BUILD_TOOLS OFF CACHE BOOL "" FORCE)
set(PF_BUILD_TESTS OFF CACHE BOOL "" FORCE)
add_subdirectory(${PF_SRC} pf)

add_executable(pf-serve pf-serve.cpp)
target_link_libraries(pf-serve PRIVATE pf)
target_compile_features(pf-serve PRIVATE cxx_std_17)
