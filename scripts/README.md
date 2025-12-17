# Cubemap Processing Scripts

## Overview
This directory contains scripts for processing panoramic images, specifically converting equirectangular panoramas to cubemap format.

## Setup

### Option 1: Python Script (Recommended)
For accurate equirectangular to cubemap conversion, install the Python dependencies:

```bash
pip install -r requirements.txt
```

The Python script provides mathematically correct projection from equirectangular to cubemap faces.

### Option 2: ImageMagick (Fallback)
If Python is not available, the system will fall back to ImageMagick. Install it using:

```bash
# macOS
brew install imagemagick

# Ubuntu/Debian
sudo apt-get install imagemagick

# CentOS/RHEL
sudo yum install ImageMagick
```

Note: ImageMagick provides approximate conversion using crop regions, which is less accurate than the Python implementation.

### Option 3: Simple Fallback
If neither Python nor ImageMagick is available, the system will use a simple fallback that copies the source image to all faces. This is not a proper conversion but ensures the service doesn't fail.

## Usage
The conversion happens automatically when processing panoramic images through the API. The system will try methods in this order:
1. Python script (most accurate)
2. ImageMagick (approximate)
3. Simple copy (fallback)

## Testing
To test the Python script manually:

```bash
python equirect_to_cubemap.py input.jpg output_dir/ 1024
```

This will create 6 face images (px.jpg, nx.jpg, py.jpg, ny.jpg, pz.jpg, nz.jpg) in the output directory.