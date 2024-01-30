import os
import shutil


buildTime = open("buildtime.log")
lines = buildTime.readlines()
i = 0
num = 0
commit = ""
flag = False
commitDict = {}
while i < len(lines):
    line = lines[i]
    if line.startswith("commit"):
        if num == 49:
            flag = True
        commitDict[commit] = flag
        commit = line.strip().replace("commit:", "")
        num = 0
        flag = False
    else:
        num += 1
    i += 1

buildTime = open("buildtime.log.v1")
lines = buildTime.readlines()
i = 0
num = 0
successCommit = []
while i < len(lines):
    line = lines[i]
    if line.startswith("commit"):
        if num == 49 and commitDict[commit] is True:
            successCommit.append(commit)
            # print(commit)
        commit = line.strip().replace("commit:", "")
        num = 0
    else:
        num += 1
    i += 1

buildlog = open("buildcache.log", "r")
lines = buildlog.readlines()
i = 0
commit = ""
folder_path = "/home/yuehang/repos/image_diff/docker_diff823/diff/"
buildcache = set()
while i < len(lines):
    line = lines[i]
    if line.startswith("commit"):
        commit = line.strip().replace("commit:", "")
    else:
        if commit in successCommit:
            tmp = line.strip().strip('(').strip(')').split(',')
            buildcache.add(tmp[1])
    i += 1

# 获取文件夹中的所有文件
all_files = os.listdir(folder_path)

# 遍历文件夹中的文件
for file in all_files:
    # 检查文件是否在set中
    if file not in buildcache:
        # 构建文件的完整路径
        file_path = os.path.join(folder_path, file)
        # 删除文件
        if os.path.isfile(file_path):
            os.remove(file_path)
        elif os.path.isdir(file_path):
            shutil.rmtree(file_path)


buildlog = open("buildcache.log.v1", "r")
lines = buildlog.readlines()
i = 0
commit = ""
folder_path = "/home/yuehang/repos/image_diff/MADE_diff823/diff/"
buildcache1 = set()
while i < len(lines):
    line = lines[i]
    if line.startswith("commit"):
        commit = line.strip().replace("commit:", "")
    else:
        if commit in successCommit:
            tmp = line.strip()
            buildcache1.add(tmp)
    i += 1

# 获取文件夹中的所有文件
all_files = os.listdir(folder_path)

# 遍历文件夹中的文件
for file in all_files:
    # 检查文件是否在set中
    if file not in buildcache1:
        # 构建文件的完整路径
        file_path = os.path.join(folder_path, file)
        # 删除文件
        if os.path.isfile(file_path):
            os.remove(file_path)
        elif os.path.isdir(file_path):
            shutil.rmtree(file_path)
            
