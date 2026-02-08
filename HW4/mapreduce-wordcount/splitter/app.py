import os
import math
import uuid
import time
import boto3
from flask import Flask, request, jsonify

app = Flask(__name__)

BUCKET = os.environ.get("S3_BUCKET", "mapreduce-wordcount-mrrs")
REGION = os.environ.get("AWS_REGION", "us-east-1")
s3 = boto3.client("s3", region_name=REGION)


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "healthy", "service": "splitter"})


@app.route("/split", methods=["GET"])
def split():
    """
    GET /split?input_key=<S3 key>&num_chunks=3
    Downloads the text file from S3, splits into N chunks by lines,
    uploads each chunk to S3, returns the chunk keys.
    """
    start_time = time.time()
    input_key = request.args.get("input_key")
    num_chunks = int(request.args.get("num_chunks", 3))

    if not input_key:
        return jsonify({"error": "input_key is required"}), 400

    # Download file from S3
    obj = s3.get_object(Bucket=BUCKET, Key=input_key)
    text = obj["Body"].read().decode("utf-8")
    lines = text.splitlines()

    # Split into roughly equal chunks by lines
    chunk_size = math.ceil(len(lines) / num_chunks)
    chunk_keys = []
    job_id = str(uuid.uuid4())[:8]

    for i in range(num_chunks):
        chunk_lines = lines[i * chunk_size : (i + 1) * chunk_size]
        chunk_text = "\n".join(chunk_lines)
        chunk_key = f"chunks/{job_id}/chunk_{i}.txt"
        s3.put_object(Bucket=BUCKET, Key=chunk_key, Body=chunk_text.encode("utf-8"))
        chunk_keys.append(chunk_key)

    elapsed = time.time() - start_time
    return jsonify({
        "job_id": job_id,
        "chunk_keys": chunk_keys,
        "num_lines": len(lines),
        "num_chunks": num_chunks,
        "elapsed_seconds": round(elapsed, 4),
    })


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)
