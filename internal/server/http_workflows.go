package server

import (
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *Server) handleWorkflowTriggers(w http.ResponseWriter, r *http.Request) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}

	resp, err := s.GetWorkflowTriggers(ctx, &emptypb.Empty{})
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}

	writeProtoJSONArray(w, frontendProtoMarshaler(), resp.Triggers)
}
