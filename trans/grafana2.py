# 打文件并读取
with open("result.txt", "r") as f:
    lines = f.readlines()

# 解析数据
image_data = []
current_image = {}
for line in lines:
    line = line.strip()
    if line.startswith("imageID:"):
        if current_image:
            image_data.append(current_image)
        current_image = {"id": line[8:], "layers": []}
    else:
        parts = line.split(",")
        current_image["layers"].append({
            "buildcache": parts[0],
            "tarsum": parts[1],
            "size": int(parts[2])
        })
if current_image:
    image_data.append(current_image)

# 计算共享率和冗余率
total_size = 0
shared_size = 0
redundant_size = 0
buildcaches = {}
tarsums = set()
for image in image_data:
    for layer in image["layers"]:
        #total_size += layer["size"]
        if layer["buildcache"] in buildcaches:
            if buildcaches[layer["buildcache"]] == 1:
                shared_size += layer["size"]
            buildcaches[layer["buildcache"]] += 1
        else:
            total_size += layer["size"]
            buildcaches[layer["buildcache"]] = 1
            if layer["tarsum"] in tarsums:
                redundant_size += layer["size"]
            else:
                tarsums.add(layer["tarsum"])

shared_rate = shared_size / total_size
redundant_rate = redundant_size / total_size

# 输出结果
print(total_size / (1024 * 1024))
print(shared_size / (1024 * 1024))
print(redundant_size / (1024 * 1024))

print("共享率：{:.2%}".format(shared_rate))
print("冗余率：{:.2%}".format(redundant_rate))
