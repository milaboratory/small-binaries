import sys

name = "world"
if len(sys.argv) > 1:
    name = sys.argv[1]

print(f'Hello, {name}!')
