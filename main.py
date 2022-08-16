from fastapi import FastAPI
from pydantic import BaseModel, constr, conlist
from typing import List
from transformers import pipeline

# Load the pipeline and model for classification
# For GPU include device=0
classifier = pipeline("zero-shot-classification", model="../models/bart-large-mnli",multi_class=True)
app = FastAPI()

class UserRequestIn(BaseModel):
    """
    Class for holding the text and labels from user request
    """
    text: constr(min_length=1)
    labels: conlist(str, min_items=1)

class ScoredLabelsOut(BaseModel):
    """
    Class for holding the labels and scores to return
    """
    labels: List[str]
    scores: List[float]

@app.post("/classification", response_model=ScoredLabelsOut)
def read_classification(user_request_in: UserRequestIn):
    """
    Process the user request with text and labels.
    Return the scores for each label according to the text
    """
    return classifier(user_request_in.text, user_request_in.labels)