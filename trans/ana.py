import re
# /var/lib/docker/vfs/dir/885c739c431d87ee6f0509c4bc39d27dc645c0a44c1cde7ee76becbb27c0327e
f = open("mkdir.log","r")
lines = f.readlines()

init_regex = r"var\/lib\/docker\/vfs\/dir\/[0-9a-zA-z]+-init\/"

regular_regax = r"var\/lib\/docker\/vfs\/dir\/[0-9a-zA-z]+\/"

for line in lines[1500:1700]:
    for arg in line.split(', '):
        matches = re.search(regular_regax, arg)
        if matches:
            if not arg.endswith(matches.group()):
                print('--------------------------------------------')
                print(arg)
                print(arg.replace(matches.group(),''))
        