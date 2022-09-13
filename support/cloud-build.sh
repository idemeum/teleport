#!/bin/bash

help()
{
   echo ""
   echo "Usage: $0 -r <aws region> -p <aws credential profile> -s <s3 bucket name> -b [publish|build|upload] "
   exit 1 # Exit script after printing help
}

s3bucketName=
#Upload file to s3
function upload() {
   releaseFile="$(basename -- $1)"
   s3FileName="s3://$s3bucketName/remote-access-releases/$releaseFile"
   echo "Copying the file to $s3FileName"
   aws --region $region --profile $profile s3 cp $1 $s3FileName
}

function cleanup() {
   echo "Deleting old binaries"
   rm -rf "idemeum-remote-access-*"
}

while getopts ":p:r:a:b:s:" opt
do
   case "$opt" in
      p ) profile="$OPTARG" ;;
      r ) region="$OPTARG" ;;
      b ) buildOption="$OPTARG" ;;
      s ) s3bucketName="$OPTARG" ;;
      #? ) help ;; # Print help in case parameter is non-existent
   esac
done

echo "Parameters: $profile $region $buildOption"
# Print helpFunction in case parameters are empty
if [ -z "$profile" ] || [ -z "$buildOption" ] || [ -z "$region" ]; then
   echo "Missing required parameters";
   help
fi

if [[ "$buildOption" == "publish" ]]; then
   cleanup
   echo "Build and publish the release binary aws s3"
   #Command to build for current platform 
   make release -C build.assets build-binaries
   for file in $(find . -name "idemeum-remote-access-*" -type f ) ; do 
      upload "$file"
   done 
elif [[ "$buildOption" == "upload" ]]; then
   cleanup
   echo "Uploading the release binary aws s3"
   #Lookup binary and upload
   for file in $(find . -name "idemeum-remote-access-*" -type f ) ; do 
      upload "$file"
   done 
else
   echo "Building the release binary"
   cleanup
   make release -C build.assets build-binaries
fi
