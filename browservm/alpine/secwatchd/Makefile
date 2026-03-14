CC = gcc
CFLAGS = -static -O2 -Wall -Wextra
TARGET = secwatchd

all: $(TARGET)

$(TARGET): main.c
	$(CC) $(CFLAGS) -o $(TARGET) main.c

clean:
	rm -f $(TARGET)

.PHONY: all clean
