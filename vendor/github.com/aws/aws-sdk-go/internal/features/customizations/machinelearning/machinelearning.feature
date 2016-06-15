# language: en
@machinelearning
Feature: Amazon Machine Learning

  I want to use Amazon Machine Learning


  Scenario: Predict API endpoint
    When I attempt to call the "Predict" API without the "PredictEndpoint" parameter
    Then the request should fail

  Scenario: Predict API endpoint
    When I attempt to call the "Predict" API with "PredictEndpoint" parameter
    Then the hostname should equal the "PredictEndpoint" parameter

  Scenario: Predict API endpoint error handling
    When I attempt to call the "Predict" API with JSON:
    """
    { "MLModelId": "fake-id", Record: {}, PredictEndpoint: "realtime.machinelearning.us-east-1.amazonaws.com" }
    """
    Then the error code should be "PredictorNotMountedException"
