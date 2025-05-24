{ pkgs ? import <nixpkgs> {
    # Pin to a specific nixpkgs commit for reproducibility
    overlays = [ ];
  }
}:

let
  # Pin to Nix 25.05 release for reproducibility
  # Using a specific commit from the nixos-25.05 branch
  pinnedPkgs = import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/55d1f923c480dadce40f5231feb472e81b0bab48.tar.gz";
    sha256 = "1y2zmqq7p3wbvqvp5k8xp8c6w3gfapc48zfjqr6p6zmqyafh6mmw";
  }) {};
  
  # Define specific versions of Go protobuf tools
  protoc-gen-go = pinnedPkgs.protoc-gen-go;
  protoc-gen-go-grpc = pinnedPkgs.protoc-gen-go-grpc;
  
in pkgs.mkShell {
  name = "immich-go-backend-dev";
  
  buildInputs = with pinnedPkgs; [
    go
    
    # Protocol Buffers compiler
    protobuf
    
    # Go protobuf plugins - pinned versions
    protoc-gen-go       # v1.32.0 or similar
    protoc-gen-go-grpc  # v1.3.0 or similar
    
    # Buf - modern Protocol Buffers tooling
    buf
    
    # Additional useful tools for gRPC development
    grpcurl      # Command-line tool for gRPC
    grpc-tools   # Additional gRPC utilities
    
    # Development utilities
    git
    curl
    jq
  ];
  
  # Set up environment variables
  shellHook = ''
    echo "🚀 Immich Go Backend Development Environment (Nix 25.05)"
    echo "📦 Available tools:"
    echo "   - Go:              $(go version)"
    echo "   - protoc:          $(protoc --version)"
    echo "   - protoc-gen-go:   $(which protoc-gen-go)"
    echo "   - protoc-gen-go-grpc: $(which protoc-gen-go-grpc)"
    echo "   - buf:             $(buf --version)"
    echo ""
    echo "🔧 To generate protobuf files:"
    echo "   buf generate"
    echo ""
    echo "🔍 To install Go dependencies:"
    echo "   go mod tidy"
    echo "   go mod download"
    
    # Ensure Go tools are in PATH
    export PATH="$GOPATH/bin:$PATH"
    
    # Create bin directory if it doesn't exist
    mkdir -p ./bin
    
    # Install specific versions of protoc plugins if needed
    # Uncomment these lines if you want to install specific versions
    # echo "📥 Installing pinned protoc-gen-go..."
    # go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32.0
    # echo "📥 Installing pinned protoc-gen-go-grpc..."
    # go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
  '';
  
  # Go environment variables
  GOPATH = "./gopath";
  GO111MODULE = "on";
  CGO_ENABLED = "0";
  
  # Protocol Buffers environment
  PROTOC_INCLUDE = "${pinnedPkgs.protobuf}/include";
}
