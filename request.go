package simpwebserv

import (
	"archive/zip"
	"bytes"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (request *Request) ConnRead(buf []byte) (int, error) {
	if !bytes.Equal(request.readRestData, []byte{}) {
		if cap(buf) <= len(request.readRestData) {
			buf = request.readRestData[0:cap(buf)]
			request.readRestData = request.readRestData[:len(request.readRestData)]
			return cap(buf), nil
		} else {
			newBuf := make([]byte, cap(buf)-len(request.readRestData))
			i, err := request.conn.Read(newBuf)
			request.bodyReaded += uint64(i)
			buf = append(request.readRestData, newBuf...)
			request.readRestData = []byte{}
			return len(buf), err
		}
	}
	if request.enableKeepAlive {
		request.conn.SetReadDeadline(time.Now().Add(time.Second * request.keepAliveTimeout))
	}
	i, err := request.conn.Read(buf)
	request.bodyReaded += uint64(i)
	return i, err
}

func (request *Request) ConnReadUntil(spliter []byte, writer io.Writer, maxSize uint64) error {
	readBuffer1 := make([]byte, bufferMaxSize)
	readBuffer2 := make([]byte, bufferMaxSize)
	var l1 int
	var l2 int
	var split [][]byte
	var err error
	for {
		l1, err = request.ConnRead(readBuffer1)
		if err != nil {
			return err
		}
		split = bytes.Split(append(readBuffer2[:l2], readBuffer1[:l1]...), spliter)
		if len(split) >= 2 {
			writer.Write(split[0])
			request.readRestData = append(request.readRestData, bytes.Join(split[1:], spliter)...)
			break
		} else {
			writer.Write(readBuffer2[:l2])
			copy(readBuffer2, readBuffer1)
			l2 = l1
		}
	}
	return nil
}

func (request *Request) ConnWrite(buf []byte) (int, error) {
	i, err := request.conn.Write(buf)
	return i, err
}

func (request *Request) SendHeader(response *Response) {
	request.conn.Write([]byte(response.Protocol + " " + response.Code + " " + response.CodeName + "\r\n"))

	for k, v := range response.Header {
		request.conn.Write([]byte(k + ": " + v + "\r\n"))
	}
	request.conn.Write([]byte("\r\n"))
	response.sendedHeader = true
}

func (request *Request) DecodeUrlParameter() map[string]string {
	parameterList := strings.Split(request.UrlParameter, "&")
	parameterMap := make(map[string]string)
	var parameterSplit []string
	var first string
	for i := 0; i < len(parameterList); i++ {
		parameterSplit = strings.Split(parameterList[i], "=")
		if len(parameterSplit) == 2 {
			first, _ = url.PathUnescape(parameterSplit[0])
			parameterMap[first], _ = url.PathUnescape(parameterSplit[1])
		}
	}
	return parameterMap
}

func (request *Request) DecodeFormUrlEncoded() (map[string]string, error) {
	if request.Method == "POST" {
		if contentType, ok := request.Header["Content-Type"]; ok {
			if contentType == "application/x-www-form-urlencoded" {
				if contentLengthString, ok := request.Header["Content-Length"]; ok {
					contentLength, err := strconv.ParseUint(contentLengthString, 10, 64)
					if err != nil {
						return nil, err
					}
					buffer := make([]byte, contentLength)
					_, err = request.ConnRead(buffer)
					if err != nil {
						return nil, err
					}
					dataMap := make(map[string]string)
					parameterList := strings.Split(string(buffer), "&")
					var parameterSplit []string
					var first string
					for i := 0; i < len(parameterList); i++ {
						parameterSplit = strings.Split(parameterList[i], "=")
						if len(parameterSplit) == 2 {
							first, _ = url.PathUnescape(parameterSplit[0])
							dataMap[first], _ = url.PathUnescape(parameterSplit[1])
						}
					}
					return dataMap, nil
				}
			}
		}
	}
	return nil, ErrRequirementNotSatisfied
}

func (request *Request) SendFile(response *Response, path string, filename string) {
	f, err := os.Open(path)
	if err != nil {
		*response = *Build404Response()
		return
	}
	defer f.Close()
	fileStat, err := f.Stat()
	if err != nil {
		*response = *Build404Response()
		return
	}

	response.Header["Content-Type"] = "application/octet-stream"
	response.Header["Content-Disposition"] = "attachment; filename=" + filename

	if fileStat.IsDir() {
		response.Header["Transfer-Encoding"] = "chunked"
		if request.Method != "HEAD" {
			pipeReader, pipeWriter := io.Pipe()
			sendBuffer := make([]byte, fileSendBufferSize)
			zipArchive := zip.NewWriter(pipeWriter)
			go func() {
				filepath.Walk(path, func(tempPath string, info os.FileInfo, _ error) error {
					if tempPath == path {
						return nil
					}
					header, _ := zip.FileInfoHeader(info)
					header.Name = strings.TrimPrefix(tempPath, path+"\\")

					if info.IsDir() {
						header.Name += "/"
					} else {
						header.Method = zip.Store
					}

					writer, _ := zipArchive.CreateHeader(header)
					if !info.IsDir() {
						file, _ := os.Open(tempPath)
						defer file.Close()
						io.Copy(writer, file)
					}
					return nil
				})
				zipArchive.Close()
				pipeWriter.Close()
			}()
			request.SendHeader(response)
			for {
				n, err := pipeReader.Read(sendBuffer)
				if err == io.EOF {
					break
				}
				if n == 0 {
					continue
				}
				_, err = request.ConnWrite([]byte(strings.ToUpper(strconv.FormatInt(int64(n), 16)) + "\r\n"))
				if err != nil {
					request.conn.Close()
					return
				}
				_, err = request.ConnWrite(sendBuffer[:n])
				if err != nil {
					request.conn.Close()
					return
				}
				_, err = request.ConnWrite([]byte("\r\n"))
				if err != nil {
					request.conn.Close()
					return
				}
			}
			_, err := request.ConnWrite([]byte("0\r\n\r\n"))
			if err != nil {
				request.conn.Close()
				return
			}
			return
		}
	} else {
		response.Header["Accept-Ranges"] = "bytes"
		fileSize := fileStat.Size()
		response.Header["Content-Length"] = strconv.FormatInt(fileSize, 10)
		if request.Method != "HEAD" {
			var ok bool
			var requestRangeString string
			if requestRangeString, ok = request.Header["Range"]; ok {
				response.Code = "206"
				response.CodeName = "Partial Content"
				requestRangeString = strings.Split(requestRangeString, "=")[1]
				requestRangeSplit := strings.Split(requestRangeString, ", ")
				var rangeSplit []string
				var startPos int64
				var endPos int64

				rangeSplit = strings.Split(requestRangeSplit[0], "-")
				startPos, err = strconv.ParseInt(rangeSplit[0], 10, 64)
				if err != nil {
					*response = *Build404DefaultResponse()
					response.Code = "416"
					response.CodeName = "Partial Content"
					return
				}

				if rangeSplit[1] == "" {
					endPos = fileSize
					rangeSplit[1] = strconv.FormatInt(fileSize, 10)
				} else {
					endPos, err = strconv.ParseInt(rangeSplit[0], 10, 64)
					if err != nil {
						*response = *Build404DefaultResponse()
						response.Code = "416"
						response.CodeName = "Partial Content"
						return
					}
				}
				if startPos > fileSize || endPos > fileSize || endPos < startPos {
					*response = *Build404DefaultResponse()
					response.Code = "416"
					response.CodeName = "Partial Content"
					return
				}
				response.Header["Content-Range"] = "bytes " + strconv.FormatInt(startPos, 10) + "-" + strconv.FormatInt(endPos-1, 10) + "/" + strconv.FormatInt(fileSize, 10)
				restDataLength := endPos - startPos
				response.Header["Content-Length"] = strconv.FormatInt(restDataLength, 10)
				f.Seek(startPos, io.SeekStart)
				buffer := make([]byte, fileSendBufferSize)
				var n int
				request.SendHeader(response)
				for {
					if restDataLength < fileSendBufferSize {
						break
					}
					n, err = f.Read(buffer)
					if err != nil {
						panic(err)
					}
					restDataLength -= int64(n)
					_, err = request.ConnWrite(buffer)
					if err != nil {
						request.conn.Close()
						return
					}
				}
				buffer = make([]byte, restDataLength)
				_, err = f.Read(buffer)
				if err != nil {
					panic(err)
				}
				_, err = request.ConnWrite(buffer)
				if err != nil {
					request.conn.Close()
					return
				}
			} else {
				request.SendHeader(response)

				restDataLength := fileSize
				buffer := make([]byte, fileSendBufferSize)
				var n int
				for {
					if restDataLength < fileSendBufferSize {
						break
					}
					n, err = f.Read(buffer)
					if err != nil {
						panic(err)
					}
					restDataLength -= int64(n)
					_, err = request.ConnWrite(buffer)
					if err != nil {
						request.conn.Close()
						return
					}
				}
				buffer = make([]byte, restDataLength)
				_, err = f.Read(buffer)
				if err != nil {
					panic(err)
				}
				_, err = request.ConnWrite(buffer)
				if err != nil {
					request.conn.Close()
					return
				}
			}
			return
		}
	}
	request.SendHeader(response)
}

func (request *Request) RecvFile(storePath string, filename string, maxSize uint64) error {
	if request.Method == "POST" {
		if contentType, ok := request.Header["Content-Type"]; ok {
			contentTypeSplit := strings.Split(contentType, "; ")
			if contentTypeSplit[0] == "multipart/form-data" {
				boundarySplit := strings.Split(contentTypeSplit[1], "=")
				boundary := boundarySplit[1]
				buffer := make([]byte, len(boundary)+4)
				_, err := request.ConnRead(buffer)
				if err != nil {
					return err
				}

				buffer = make([]byte, 1)
				recv := new(bytes.Buffer)
				var first string
				var second string
				var i int
				partHeaderMap := make(map[string]string)
				for {
					if request.enableKeepAlive {
						request.conn.SetReadDeadline(time.Now().Add(time.Second * request.keepAliveTimeout))
					}
					httpReadHeaderLine(request.conn, &buffer, recv, &first, &second, &err, &i)
					if err != nil {
						return err
					}
					if first == "" || second == "" {
						break
					}
					partHeaderMap[first] = second
				}
				if contentDisposition, ok := partHeaderMap["Content-Disposition"]; ok {
					contentDispositionSplit := strings.Split(contentDisposition, "; ")
					contentDispositionSplit = contentDispositionSplit[1:]
					var attributeSplit []string
					attributeMap := make(map[string]string)
					for i = 0; i < len(contentDispositionSplit); i++ {
						attributeSplit = strings.Split(contentDispositionSplit[i], "=")
						first, _ = url.PathUnescape(attributeSplit[0])
						second, _ = url.PathUnescape(attributeSplit[1][1 : len(attributeSplit[1])-1])
						attributeMap[first] = second
					}
					if filename == "" {
						filename = attributeMap["filename"]
					}
					f, err := os.Create(storePath + "/" + filename)
					if err != nil {
						return err
					}
					defer f.Close()
					err = request.ConnReadUntil([]byte("\r\n--"+boundary), f, maxSize)
					return err
				}
			}
		}
	}
	return ErrRequirementNotSatisfied
}
