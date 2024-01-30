import docker
from collections import defaultdict

client = docker.from_env()

tarSumDict = defaultdict(str)

tarSum = open("tarsum.log.v1","r")
lines = tarSum.readlines()
for line in lines:
    tmp = line.strip().strip('(').strip(')').split(',')
    tarSumDict[tmp[0]] = tmp[1]

buildlog = open("buildcache.log.v1", "r")
lines = buildlog.readlines()
i = 0
commit = ""
image_id = ""
tarSumList = []
tmpDict = defaultdict(str)
commitDict = {}
while i < len(lines):
    line = lines[i]
    if line.startswith("commit"):
        commitDict[image_id] = tarSumList
        tarSumList = []
        commit = line.strip().replace("commit:","")
        image_id = client.images.get(f'grafana:{commit}').id
    else :
        tmp = line.strip().strip('(').strip(')').split(',')
        if tmpDict[tmp[0]] == "":
            tmpDict[tmp[0]] = tarSumDict[tmp[1]]
        tarSumList.append(tmpDict[tmp[0]])
    i += 1
commitDict[image_id] = tarSumList
#print(tmpDict)

layerlog = open("grafana_history.txt", "r")
result = open("result.txt", "w")
lines = layerlog.readlines()
tmpDict1 = defaultdict(str)
for line in lines:
    if line.startswith("Image"):
        image_id = line.strip().replace("Image ID: ","")
        result.write(f"imageID:{image_id}\n")
    else:
        tmp = line.strip().split(',')
        result.write(f"{tmp[0]},{tmpDict['sha256:'+tmp[0]]},{tmp[1]}\n")

