while read line; do
  git reset --hard $line
  echo "commit:$line" >> /go/src/github.com/docker/docker/trans/buildcache.log
  while true; do
	  echo "commit:$line" >> /go/src/github.com/docker/docker/trans/buildtime.log
	  docker build --build-arg HTTP_PROXY=http://202.114.7.81:7890 \
		  --build-arg HTTPS_PROXY=http://202.114.7.81:7890 \
		  -f Dockerfile -t jumpserver:$line ./
	  if [ $? -eq 0 ]
	  then
		  break
	  else
		  echo "failed" >> /go/src/github.com/docker/docker/trans/buildtime.log
	  fi
  done
done < commits20.txt
