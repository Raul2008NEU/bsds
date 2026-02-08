import os
import re
import json
import time
import boto3
from flask import Flask, request, jsonify

app = Flask(__name__)

BUCKET = os.environ.get("S3_BUCKET", "mapreduce-wordcount-mrrs")
REGION = os.environ.get("AWS_REGION", "us-east-1")
s3 = boto3.client("s3", region_name=REGION)


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "healthy", "service": "mapper"})


@app.route("/map", methods=["GET"])
def map_words():
    """
    GET /map?chunk_key=<S3 key>
    Downloads a text chunk from S3, counts word occurrences,
    saves JSON results to S3, returns the output key.
    """
    start_time = time.time()
    chunk_key = request.args.get("chunk_key")

    if not chunk_key:
        return jsonify({"error": "chunk_key is required"}), 400

    # Download chunk from S3
    obj = s3.get_object(Bucket=BUCKET, Key=chunk_key)
    text = obj["Body"].read().decode("utf-8")

    # Count words (lowercase, strip punctuation)
    word_counts = {}
    words = re.findall(r"[a-zA-Z']+", text.lower())
    for word in words:
        word = word.strip("'")  # remove leading/trailing apostrophes
        if word:
            word_counts[word] = word_counts.get(word, 0) + 1

    # Sort by count descending for readability
    sorted_counts = dict(sorted(word_counts.items(), key=lambda x: -x[1]))

    # Upload results to S3
    output_key = chunk_key.replace("chunks/", "mapped/").replace(".txt", ".json")
    s3.put_object(
        Bucket=BUCKET,
        Key=output_key,
        Body=json.dumps(sorted_counts, indent=2).encode("utf-8"),
        ContentType="application/json",
    )

    elapsed = time.time() - start_time
    return jsonify({
        "output_key": output_key,
        "unique_words": len(sorted_counts),
        "total_words": sum(sorted_counts.values()),
        "elapsed_seconds": round(elapsed, 4),
    })


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)
