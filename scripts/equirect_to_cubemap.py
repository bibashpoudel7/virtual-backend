#!/usr/bin/env python3
"""
Convert equirectangular panorama to cubemap faces.
Usage: python equirect_to_cubemap.py <input_image> <output_dir> <face_size>
"""

import sys
import os
import numpy as np
from PIL import Image
import math

def equirect_to_cubemap(img_path, output_dir, face_size=1024):
    """Convert equirectangular image to 6 cubemap faces."""
    
    # Load the equirectangular image
    equirect = Image.open(img_path)
    equirect_array = np.array(equirect)
    h, w = equirect_array.shape[:2]
    
    # Face names and their corresponding rotations
    faces = {
        'px': (np.pi/2, 0, 0),    # positive X (right)
        'nx': (-np.pi/2, 0, 0),   # negative X (left)
        'py': (0, np.pi/2, 0),     # positive Y (top)
        'ny': (0, -np.pi/2, 0),    # negative Y (bottom)
        'pz': (0, 0, 0),           # positive Z (front)
        'nz': (np.pi, 0, 0),       # negative Z (back)
    }
    
    os.makedirs(output_dir, exist_ok=True)
    
    for face_name, (yaw, pitch, roll) in faces.items():
        face = extract_cubemap_face(equirect_array, face_size, yaw, pitch)
        face_img = Image.fromarray(face)
        output_path = os.path.join(output_dir, f"{face_name}.jpg")
        face_img.save(output_path, 'JPEG', quality=90)
        print(f"Saved {face_name} to {output_path}")

def extract_cubemap_face(equirect, face_size, yaw, pitch):
    """Extract a single cubemap face from equirectangular image."""
    
    h, w = equirect.shape[:2]
    face = np.zeros((face_size, face_size, 3), dtype=np.uint8)
    
    # Create coordinate grids for the face
    x = np.linspace(-1, 1, face_size)
    y = np.linspace(-1, 1, face_size)
    X, Y = np.meshgrid(x, y)
    
    # Convert face coordinates to 3D vectors
    if abs(yaw) < 0.1 and abs(pitch) < 0.1:  # Front face (pz)
        vx, vy, vz = X, -Y, np.ones_like(X)
    elif abs(yaw - np.pi) < 0.1:  # Back face (nz)
        vx, vy, vz = -X, -Y, -np.ones_like(X)
    elif abs(yaw - np.pi/2) < 0.1:  # Right face (px)
        vx, vy, vz = np.ones_like(X), -Y, -X
    elif abs(yaw + np.pi/2) < 0.1:  # Left face (nx)
        vx, vy, vz = -np.ones_like(X), -Y, X
    elif pitch > 0:  # Top face (py)
        vx, vy, vz = X, np.ones_like(X), Y
    else:  # Bottom face (ny)
        vx, vy, vz = X, -np.ones_like(X), -Y
    
    # Normalize vectors
    norm = np.sqrt(vx*vx + vy*vy + vz*vz)
    vx, vy, vz = vx/norm, vy/norm, vz/norm
    
    # Convert to spherical coordinates
    theta = np.arctan2(vx, vz)  # Longitude
    phi = np.arcsin(np.clip(vy, -1, 1))  # Latitude
    
    # Map to equirectangular coordinates
    u = (theta + np.pi) / (2 * np.pi)
    v = (phi + np.pi/2) / np.pi
    
    # Convert to pixel coordinates
    px = (u * w).astype(int)
    py = (v * h).astype(int)
    
    # Clamp coordinates
    px = np.clip(px, 0, w-1)
    py = np.clip(py, 0, h-1)
    
    # Sample from equirectangular image with correct orientation
    for i in range(face_size):
        for j in range(face_size):
            # Flip vertically to fix upside-down issue
            face[face_size - 1 - i, j] = equirect[py[i, j], px[i, j]]
    
    return face

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python equirect_to_cubemap.py <input_image> <output_dir> [face_size]")
        sys.exit(1)
    
    input_image = sys.argv[1]
    output_dir = sys.argv[2]
    face_size = int(sys.argv[3]) if len(sys.argv) > 3 else 1024
    
    if not os.path.exists(input_image):
        print(f"Error: Input file {input_image} not found")
        sys.exit(1)
    
    try:
        equirect_to_cubemap(input_image, output_dir, face_size)
        print("Conversion completed successfully")
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)