mockery --recursive=true --all --dir=../service --output=.
mockery --recursive=true --all --dir=../repository --output=.
mockery --recursive=true --all --dir=../database --output=.
mockery --recursive=true --all --dir=../payment_system --output=.
mockery --recursive=true --name=BillingService --dir=../../pkg/proto/grpc --output=../../pkg/mocks
