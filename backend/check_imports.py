try:
    from huggingface_hub import HfApi
    print("Import HfApi SUCCESS")
    api = HfApi()
    print("Instantiate HfApi SUCCESS")
except ImportError as e:
    print(f"ImportError: {e}")
except Exception as e:
    print(f"Error: {e}")
