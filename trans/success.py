from collections import defaultdict


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
commit = ""
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

tarsumDict = defaultdict(str)
tarSumDict = defaultdict(str)

tarSum = open("tarsum.log", "r")
lines = tarSum.readlines()
for line in lines:
    tmp = line.strip().strip('(').strip(')').split(',')
    tarsumDict[tmp[0]] = tmp[1]

tarSum = open("tarsum.log.v1", "r")
lines = tarSum.readlines()
for line in lines:
    tmp = line.strip().strip('(').strip(')').split(',')
    tarSumDict[tmp[0]] = tmp[1]

buildlog = open("buildcache.log", "r")
lines = buildlog.readlines()
i = 0
commit = ""
tarSumList = []
tmpDict = defaultdict(str)
commitDict = {}
while i < len(lines):
    line = lines[i]
    if line.startswith("commit"):
        if commit in successCommit:
            commitDict[commit] = tarSumList
        tarSumList = []
        commit = line.strip().replace("commit:", "")
    else:
        tmp = line.strip().strip('(').strip(')').split(',')
        if tmpDict[tmp[0]] == "":
            tmpDict[tmp[0]] = tarsumDict[tmp[1]]
        tarSumList.append(tmpDict[tmp[0]])
    i += 1
commitDict[commit] = tarSumList

buildlog = open("buildcache.log.v1", "r")
lines = buildlog.readlines()
commit = ""
i = 0
while i < len(lines):
    line = lines[i]
    if line.startswith("commit"):
        if i != 0:
            if commit in successCommit:
                print(commit)
                for layer in tarSumList[-3:]:
                    if not layer in commitDict[commit]:
                        print(layer)
        tarSumList = []
        commit = line.strip().replace("commit:", "")
    else:
        tmp = line.strip()
        tarSumList.append(tarSumDict[tmp])
    i += 1

