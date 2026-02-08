import os
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
    return jsonify({"status": "healthy", "service": "reducer"})


@app.route("/reduce", methods=["GET"])
def reduce_words():
    """
    GET /reduce?mapped_keys=key1,key2,key3
    Downloads mapped JSON files from S3, aggregates word counts,
    saves final result to S3, returns the output key.
    """
    start_time = time.time()
    mapped_keys_param = request.args.get("mapped_keys")

    if not mapped_keys_param:
        return jsonify({"error": "mapped_keys is required (comma-separated)"}), 400

    mapped_keys = [k.strip() for k in mapped_keys_param.split(",")]

    # Aggregate word counts from all mappers
    final_counts = {}
    for key in mapped_keys:
        obj = s3.get_object(Bucket=BUCKET, Key=key)
        partial = json.loads(obj["Body"].read().decode("utf-8"))
        for word, count in partial.items():
            final_counts[word] = final_counts.get(word, 0) + count

    # Sort by count descending
    sorted_counts = dict(sorted(final_counts.items(), key=lambda x: -x[1]))

    # Extract job_id from first key: mapped/<job_id>/chunk_0.json
    parts = mapped_keys[0].split("/")
    job_id = parts[1] if len(parts) > 1 else "unknown"

    output_key = f"results/{job_id}/final_counts.json"
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
