// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"
import (
	"fmt"
	"os"
	"reflect"
	"time"
	"unsafe"

	"storj.io/uplink"
)

// Upload is a partial upload to Storj Network.
type Upload struct {
	scope
	upload *uplink.Upload
    logfile *os.File
}

const UPLINK_LOG = "UPLINK_LOG"

func uplink_log_file() *os.File {
	var logfile *os.File
    var err error
	uplinklog := os.Getenv(UPLINK_LOG)
	if uplinklog != "" {
        logfile, err = os.OpenFile(uplinklog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
        if err != nil {
            return nil
        }
    }
	return logfile
}

// uplink_upload_object starts an upload to the specified key.
//
//export uplink_upload_object
func uplink_upload_object(project *C.UplinkProject, bucket_name, object_key *C.uplink_const_char, options *C.UplinkUploadOptions) C.UplinkUploadResult { //nolint:golint
	logfile := uplink_log_file()
	if logfile != nil {
		defer logfile.Close()
	}

	if project == nil {
		if logfile != nil {
			_, err := logfile.WriteString(fmt.Sprintf("uplink_upload_object: project is nil\n"))
			if err != nil {
				return C.UplinkUploadResult{
					error: mallocError(err),
				}
			}
		}
		return C.UplinkUploadResult{
			error: mallocError(ErrNull.New("project")),
		}
	}
	if bucket_name == nil {
		if logfile != nil {
			_, err := logfile.WriteString(fmt.Sprintf("uplink_upload_object: bucket_name is nil\n"))
			if err != nil {
				return C.UplinkUploadResult{
					error: mallocError(err),
				}
			}
		}
		return C.UplinkUploadResult{
			error: mallocError(ErrNull.New("bucket_name")),
		}
	}
	if object_key == nil {
		if logfile != nil {
			_, err := logfile.WriteString(fmt.Sprintf("uplink_upload_object: object_key is nil\n"))
			if err != nil {
				return C.UplinkUploadResult{
					error: mallocError(err),
				}
			}
		}
		return C.UplinkUploadResult{
			error: mallocError(ErrNull.New("object_key")),
		}
	}

	proj, ok := universe.Get(project._handle).(*Project)
	if !ok {
		if logfile != nil {
			_, err := logfile.WriteString(fmt.Sprintf("uplink_upload_object: failed to retrieve project\n"))
			if err != nil {
				return C.UplinkUploadResult{
					error: mallocError(err),
				}
			}
		}
		return C.UplinkUploadResult{
			error: mallocError(ErrInvalidHandle.New("project")),
		}
	}
	scope := proj.scope.child()

	opts := &uplink.UploadOptions{}
	if options != nil {
		if options.expires > 0 {
			opts.Expires = time.Unix(int64(options.expires), 0)
		}
	}

	upload, err := proj.UploadObject(scope.ctx, C.GoString(bucket_name), C.GoString(object_key), opts)
	if err != nil {
		if logfile != nil {
			_, err := logfile.WriteString(fmt.Sprintf("%+v\n", err))
			if err != nil {
				return C.UplinkUploadResult{
					error: mallocError(err),
				}
			}
		}
		return C.UplinkUploadResult{
			error: mallocError(err),
		}
	}

	if logfile != nil {
		_, err := logfile.WriteString(fmt.Sprintf("Created upload object for object key '%s'. Target bucket: '%s'\n", C.GoString(object_key), C.GoString(bucket_name)))
		if err != nil {
			return C.UplinkUploadResult{
				error: mallocError(err),
			}
		}
	}

	return C.UplinkUploadResult{
		upload: (*C.UplinkUpload)(mallocHandle(universe.Add(&Upload{scope, upload, logfile}))),
	}
}

// uplink_upload_write uploads len(p) bytes from p to the object's data stream.
// It returns the number of bytes written from p (0 <= n <= len(p)) and
// any error encountered that caused the write to stop early.
//
//export uplink_upload_write
func uplink_upload_write(upload *C.UplinkUpload, bytes unsafe.Pointer, length C.size_t) C.UplinkWriteResult {
	logfile := uplink_log_file()
	if logfile != nil {
		defer logfile.Close()
	}

	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		if logfile != nil {
			_, err := logfile.WriteString(fmt.Sprintf("uplink_upload_write: failed to retrieve upload object\n"))
			if err != nil {
				return C.UplinkWriteResult{
					error: mallocError(err),
				}
			}
		}
		return C.UplinkWriteResult{
			error: mallocError(ErrInvalidHandle.New("upload")),
		}
	}

	ilength, ok := safeConvertToInt(length)
	if !ok {
		if logfile != nil {
			_, err := logfile.WriteString(fmt.Sprintf("uplink_upload_write: length is too large\n"))
			if err != nil {
				return C.UplinkWriteResult{
					error: mallocError(err),
				}
			}
		}
		return C.UplinkWriteResult{
			error: mallocError(ErrInvalidArg.New("length too large")),
		}
	}

	var buf []byte
	hbuf := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
	hbuf.Data = uintptr(bytes)
	hbuf.Len = ilength
	hbuf.Cap = ilength

	n, err := up.upload.Write(buf)
	if err != nil {
		if logfile != nil {
			_, err := logfile.WriteString(fmt.Sprintf("%+v\n", err))
			if err != nil {
				return C.UplinkWriteResult{
					error: mallocError(err),
				}
			}
		}
	}

	if logfile != nil {
		_, err := logfile.WriteString(fmt.Sprintf("Uploaded '%d' bytes\n", n))
		if err != nil {
			return C.UplinkWriteResult{
				error: mallocError(err),
			}
		}
	}

	return C.UplinkWriteResult{
		bytes_written: C.size_t(n),
		error:         mallocError(err),
	}
}

// uplink_upload_commit commits the uploaded data.
//
//export uplink_upload_commit
func uplink_upload_commit(upload *C.UplinkUpload) *C.UplinkError {
	logfile := uplink_log_file()
	if logfile != nil {
		defer logfile.Close()
	}

	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		if logfile != nil {
			_, err := up.logfile.WriteString(fmt.Sprintf("uplink_upload_commit: failed to retrieve upload object\n"))
			if err != nil {
				return mallocError(err)
			}
		}
		return mallocError(ErrInvalidHandle.New("upload"))
	}

	err := up.upload.Commit()
	if err != nil {
		if logfile != nil {
			_, err := up.logfile.WriteString(fmt.Sprintf("%+v", err))
			if err != nil {
				return mallocError(err)
			}
		}
	}
	return mallocError(err)
}

// uplink_upload_abort aborts an upload.
//
//export uplink_upload_abort
func uplink_upload_abort(upload *C.UplinkUpload) *C.UplinkError {
	logfile := uplink_log_file()
	if logfile != nil {
		defer logfile.Close()
	}

	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		if logfile != nil {
			_, err := up.logfile.WriteString(fmt.Sprintf("uplink_upload_abort: failed to retrieve upload object\n"))
			if err != nil {
				return mallocError(err)
			}
		}
		return mallocError(ErrInvalidHandle.New("upload"))
	}

	err := up.upload.Abort()
	if err != nil {
		if up.logfile != nil {
			_, err := up.logfile.WriteString(fmt.Sprintf("%+v", err))
			if err != nil {
				return mallocError(err)
			}
		}
	}
	return mallocError(err)
}

// uplink_upload_info returns the last information about the uploaded object.
//
//export uplink_upload_info
func uplink_upload_info(upload *C.UplinkUpload) C.UplinkObjectResult {
	logfile := uplink_log_file()
	if logfile != nil {
		defer logfile.Close()
	}

	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		if logfile != nil {
			_, err := up.logfile.WriteString(fmt.Sprintf("uplink_upload_info: failed to retrieve upload object\n"))
			if err != nil {
				return C.UplinkObjectResult{
					error: mallocError(err),
				}
			}
		}
		return C.UplinkObjectResult{
			error: mallocError(ErrInvalidHandle.New("upload")),
		}
	}

	info := up.upload.Info()
	return C.UplinkObjectResult{
		object: mallocObject(info),
	}
}

// uplink_upload_set_custom_metadata returns the last information about the uploaded object.
//
//export uplink_upload_set_custom_metadata
func uplink_upload_set_custom_metadata(upload *C.UplinkUpload, custom C.UplinkCustomMetadata) *C.UplinkError {
	logfile := uplink_log_file()
	if logfile != nil {
		defer logfile.Close()
	}

	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		if logfile != nil {
			_, err := up.logfile.WriteString(fmt.Sprintf("uplink_upload_set_custom_metadata: failed to retrieve upload object\n"))
			if err != nil {
				return mallocError(err)
			}
		}
		return mallocError(ErrInvalidHandle.New("upload"))
	}

	customMetadata := customMetadataFromC(custom)
	err := up.upload.SetCustomMetadata(up.scope.ctx, customMetadata)

	return mallocError(err)
}

// uplink_free_write_result frees any resources associated with write result.
//
//export uplink_free_write_result
func uplink_free_write_result(result C.UplinkWriteResult) {
	uplink_free_error(result.error)
}

// uplink_free_upload_result closes the upload and frees any associated resources.
//
//export uplink_free_upload_result
func uplink_free_upload_result(result C.UplinkUploadResult) {
	uplink_free_error(result.error)
	freeUpload(result.upload)
}

func freeUpload(upload *C.UplinkUpload) {
	if upload == nil {
		return
	}
	defer C.free(unsafe.Pointer(upload))
	defer universe.Del(upload._handle)

	up, ok := universe.Get(upload._handle).(*Upload)
	if ok {
		up.cancel()
	}
}
