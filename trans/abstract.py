with open("git_commit_ids.txt", "r") as f:
    lines = f.readlines()
    i = 380
    c = i - 100
    result = []
    while i > c:
        result.append(lines[i])
        i -= 1
f.close()

with open("netedata_commits.txt", "w") as ref:
    for line in result:
        ref.write("%s" % line)
ref.close()
