#include "printLib.hxx"

namespace PrintLib {
    void
    printNumber(u32 number) {
        __attribute__((import_module("host"), import_name("print_number"))) void __print_number(u32 number);
        __print_number(number);
    }

    void
    printString(u32 offset, u32 length) {
        __attribute__((import_module("host"), import_name("print_string"))) void __print_string(u32 offset, u32 length);
        __print_string(offset, length);
    }
} // namespace PrintLib
