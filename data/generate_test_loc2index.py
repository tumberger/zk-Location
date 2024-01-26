import subprocess
import re
import numpy as np
import struct

def execute_command(cmd):
    output = subprocess.run(cmd, capture_output=True, text=True)
    if output.returncode != 0:
        raise Exception(f"Command failed: {output.stderr}")
    return output.stdout.strip()

def execute_lat_lng_to_ijk(resolution, latitude, longitude):
    lat_str = f"{latitude:.30f}"
    lng_str = f"{longitude:.30f}"
    res_str = str(resolution)

    executable_path = "../h3-master/build/bin/latLngToCell"
    cmd = [executable_path, "--resolution", res_str, "--latitude", lat_str, "--longitude", lng_str]

    output = subprocess.run(cmd, capture_output=True, text=True)
    if output.returncode != 0:
        raise Exception(f"Command failed: {output.stderr}")

    pattern = r"I: (\d+), J: (\d+), K: (\d+)"
    matches = re.search(pattern, output.stdout)
    if not matches:
        raise Exception("Failed to parse output")

    return int(matches.group(1)), int(matches.group(2)), int(matches.group(3))


def lat_lng_to_cell(resolution, latitude, longitude):
    executable_path = "../h3-master/build/bin/latLngToCell"
    cmd = [executable_path, "--resolution", str(resolution), "--latitude", f"{latitude:.30f}", "--longitude", f"{longitude:.30f}"]
    output = execute_command(cmd)
    # Extract the H3 index in hexadecimal format
    h3_index = re.search(r"--- (\w+)", output)
    if not h3_index:
        raise Exception("Failed to parse H3 index")
    return h3_index.group(1)

def cell_to_lat_lng(h3_index):
    cmd = ["../h3-master/build/bin/cellToLatLng", "--index", h3_index]
    output = execute_command(cmd)
    return tuple(map(float, output.split()))

def cell_to_boundary(h3_index):
    cmd = ["../h3-master/build/bin/cellToBoundary", "--index", h3_index]
    output = execute_command(cmd)
    
    pattern = r"(\d+\.\d+)\s+(\d+\.\d+)"
    return re.findall(pattern, output)

def haversine(lat1, lon1, lat2, lon2):
    # Radius of Earth in kilometers. Use 6371 for kilometers 
    R = 6371000.0

    # Convert latitude and longitude from degrees to radians
    lat1_rad = np.radians(lat1)
    lon1_rad = np.radians(lon1)
    lat2_rad = np.radians(lat2)
    lon2_rad = np.radians(lon2)

    # Difference in coordinates
    dlat = lat2_rad - lat1_rad
    dlon = lon2_rad - lon1_rad

    # Haversine formula
    a = np.sin(dlat / 2)**2 + np.cos(lat1_rad) * np.cos(lat2_rad) * np.sin(dlon / 2)**2
    c = 2 * np.arctan2(np.sqrt(a), np.sqrt(1 - a))

    distance = R * c
    return distance

def generate_points(resolution):
    points = []
    for res in range(0, resolution + 1):
        # Convert latitude and longitude to H3 cell index
        h3_index = lat_lng_to_cell(res, 40.689167, 33.044444)  # Example coordinates in degrees

        center_lat, center_lng = cell_to_lat_lng(h3_index)  # These are in degrees

        # Convert from degrees to radians
        center_lat_rad = np.radians(center_lat)
        center_lng_rad = np.radians(center_lng)

        boundary = cell_to_boundary(h3_index)

        if not boundary:
            continue

        target_lat, target_lng = map(float, boundary[0])  # These are in degrees

        # Convert from degrees to radians
        target_lat_rad = np.radians(target_lat)
        target_lng_rad = np.radians(target_lng)

        for i in range(2, 11):
            factor = np.log(i) / np.log(20)
            gen_lat_rad = center_lat_rad + (target_lat_rad - center_lat_rad) * factor
            gen_lng_rad = center_lng_rad + (target_lng_rad - center_lng_rad) * factor

            gen_lat = center_lat + (target_lat - center_lat) * factor
            gen_lng = center_lng + (target_lng - center_lng) * factor
            
            # Convert generated point back to H3 cell index
            gen_h3_index = execute_lat_lng_to_ijk(res, gen_lat, gen_lng)
            points.append((gen_lat_rad, gen_lng_rad, res, gen_h3_index[0], gen_h3_index[1], gen_h3_index[2]))

            # Calculate and print the distance between generated point and boundary point            
            distance = haversine(gen_lat, gen_lng, target_lat, target_lng)
            print(f"Generated Point", gen_lat, gen_lng)
            print(f"Boundary Point", target_lat, target_lng)
            print(f"Distance between generated point and boundary point: {distance:.2f} meters")
    return points

def float32_to_hex(f):
    # Convert a float32 number to hexadecimal
    packed = struct.pack('>f', f)
    return struct.unpack('>I', packed)[0]

def float64_to_hex(f):
    # Convert a float32 number to hexadecimal
    packed = struct.pack('>d', f)
    return struct.unpack('>Q', packed)[0]

def int_to_hex(i):
    # Convert an integer to hexadecimal
    return i
def write_to_file(data, use_float32=False):
    if use_float32:
        file_path = './f32/loc2index32.txt'
        to_hex = float32_to_hex  # Use the float32_to_hex function
        hex_format = '{:08X}'  # Format for 32-bit hex
    else:
        file_path = './f64/loc2index64.txt'
        to_hex = float64_to_hex  # Use the float64_to_hex function
        hex_format = '{:016X}'  # Format for 64-bit hex

    with open(file_path, 'w') as file:
        for lat, lng, res, i, j, k in data:
            lat_hex = to_hex(np.float32(lat) if use_float32 else np.float64(lat))
            lng_hex = to_hex(np.float32(lng) if use_float32 else np.float64(lng))
            res_hex = int_to_hex(res)
            i_hex = int_to_hex(i)
            j_hex = int_to_hex(j)
            k_hex = int_to_hex(k)
            file.write(f"{hex_format.format(lat_hex)} {hex_format.format(lng_hex)} {res_hex:X} {i_hex:X} {j_hex:X} {k_hex:X}\n")

# Generate points for resolutions 1-15
try:
    generated_points = generate_points(15)

    # Choose whether to use float32 or float64 precision
    # Set use_float32 to True for float32 precision, False for float64
    use_float32 = False  # or False

    write_to_file(generated_points, use_float32)
    precision_type = '32' if use_float32 else '64'
    print(f"Data generation complete. Data saved to loc2index{precision_type}.txt.")
except Exception as e:
    print(f"Error: {e}")
