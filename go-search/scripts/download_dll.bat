@echo off
echo Downloading ONNX Runtime 1.16.3 for Windows x64...
curl -L -o onnxruntime.zip https://github.com/microsoft/onnxruntime/releases/download/v1.16.3/onnxruntime-win-x64-1.16.3.zip
echo Extracting...
powershell -Command "Expand-Archive -Path onnxruntime.zip -DestinationPath . -Force"
copy onnxruntime-win-x64-1.16.3\lib\onnxruntime.dll .
echo Done. onnxruntime.dll is in the current directory.
del onnxruntime.zip
rd /s /q onnxruntime-win-x64-1.16.3

