#include "printLib.hxx"

const char *msg = "Hello from C++ code!\nThe number above is the message's length.\n";

extern "C" void
_start() {
    PrintLib::u32 size = 0;
    for (const char *p = msg; *p != '\0'; ++p) {
        size++;
    }
    PrintLib::printNumber(size);
    PrintLib::printString(reinterpret_cast<PrintLib::u32>(msg), size);
}
