import grpc

import pricing_pb2_grpc


def stub(target):
    return pricing_pb2_grpc.PricingStub(grpc.insecure_channel(target))
