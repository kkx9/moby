def save_set_to_file(file_path, my_set):
    with open(file_path, 'w') as file:
        for item in my_set:
            file.write(str(item) + '\n')


layer_path = "/home/yuehang/repos/image_diff/docker_diff_test/layers/"
id_set = set()
buildlog = open("buildcache.log", "r")
lines = buildlog.readlines()[-5:]
i = 0
while i < len(lines):
    line = lines[i]
    tmp = line.strip().strip('(').strip(')').split(',')
    print(tmp[1])
    with open(layer_path + tmp[1], 'r') as file:
        for line in file:
            item = line.strip()
            id_set.add(item)
    file.close()
    id_set.add(tmp[1])
    i += 1
save_set_to_file('id.txt', id_set)
