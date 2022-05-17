#!/bin/bash
#
# A script to run a test in a loop to see if it is flakey

cd ably
for ((i=1;i<=100;i++)); 
do 
   echo $i
   go test -v -run TestStats_Direction_RSC6b2
   go clean -testcache
done
