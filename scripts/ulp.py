import math

def binary_to_float64(sign_bit, exponent_str, mantissa_str):
    """
    Convert binary strings of sign, exponent, and mantissa to a float64 number (IEEE 754).
    
    Args:
        sign_bit (str): Binary string for the sign ('0' for positive, '1' for negative).
        exponent_str (str): Binary string for the exponent (11 bits).
        mantissa_str (str): Binary string for the mantissa (up to 52 bits).

    Returns:
        float: The corresponding float64 value.
    """
    # Restrict mantissa to 52 bits and add the hidden leading bit
    mantissa_str = '1' + mantissa_str[:52]

    # Convert binary strings to integers
    sign = (-1) ** int(sign_bit)
    exponent = int(exponent_str, 2)
    mantissa = int(mantissa_str, 2)

    # Calculate the value using IEEE 754 format
    value = sign * 2 ** (exponent - 1023) * (mantissa / (2 ** 52))
    return value

def find_surrounding_floats(r):
    """
    Find the closest floating-point numbers around a given float64 value.
    
    Args:
        r (float): Input float64 number.

    Returns:
        tuple: Two floats (r_minus, r_plus) where:
               - r_minus is the largest float less than r.
               - r_plus is the smallest float greater than r.
    """
    r_minus = math.nextafter(r, -float('inf'))
    r_plus = math.nextafter(r, float('inf'))
    return r_minus, r_plus

# Input binary components for computed and wanted values
sign_computed = '0'
exponent_computed_str = "11000001100"
mantissa_computed_str = "11010000011011100011100100000000100100000001011000111"

sign_wanted = '0'
exponent_wanted_str = "11000001100"
mantissa_wanted_str = "11010000011011100011100100000000100100000001011000110"

# Convert to float64 values
computed_value = binary_to_float64(sign_computed, exponent_computed_str, mantissa_computed_str)
wanted_value = binary_to_float64(sign_wanted, exponent_wanted_str, mantissa_wanted_str)

print("Computed Value:", computed_value)
print("Wanted Value:", wanted_value)

# Find the closest floats around the wanted value
r_minus, r_plus = find_surrounding_floats(wanted_value)

# Calculate ULP
ulp_distance = r_plus - r_minus
ulp = abs(computed_value - wanted_value) / ulp_distance

print("ULP Distance (r_plus - r_minus):", ulp_distance)
print("ULP Difference (in ULPs):", ulp)
