package api

import (
	"errors"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/queryutil"
)

func sendErrorResponse(ctx *httputil.RequestContext, statusCode int, message string) {
	_ = ctx.Error(errors.New(message), statusCode)
}

// defaultPagination provides consistent pagination settings for API endpoints
func defaultPagination() queryutil.Pagination {
	return queryutil.Pagination{
		Start:    0,
		End:      10,
		PageSize: 10,
	}
}

const exampleCID = "QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D"
