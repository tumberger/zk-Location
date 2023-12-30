import struct
import math

def binary_to_float64(exponent_str, mantissa_str):
    """Convert binary strings of exponent and mantissa to a float64 number."""
    # Add the hidden leading bit to the mantissa
    mantissa_str = '1' + mantissa_str

    # Convert binary strings to integers
    exponent = int(exponent_str, 2)
    mantissa = int(mantissa_str, 2)

    # Calculate the value using IEEE 754 format
    value = (-1) ** 0 * 2 ** (exponent - 1023) * (mantissa / (2 ** 52))
    return value

def find_surrounding_floats(r):
    r_minus = math.nextafter(r, -float('inf'))  # The largest float less than r
    r_plus = math.nextafter(r, float('inf'))    # The smallest float greater than r
    return r_minus, r_plus

# SINE values
exponent_computed_str = "11000001100"
exponent_wanted_str = "11000001100"
mantissa_computed_str = "11010000011011100011100100000000100100000001011000111"
mantissa_wanted_str = "11010000011011100011100100000000100100000001011000110"

# ATAN test with pretty normal floats
# exponent_computed_str = "00000000000"
# exponent_wanted_str = "00000000000"
# mantissa_computed_str = "11000111110101000110000001100011100110011101110000001"
# mantissa_wanted_str = "11000111110101000110000001100011011111100110101110111"


# Convert to float64 values
computed_value = binary_to_float64(exponent_computed_str, mantissa_computed_str)
wanted_value = binary_to_float64(exponent_wanted_str, mantissa_wanted_str)

print(computed_value)
print(wanted_value)

r_minus, r_plus = find_surrounding_floats(wanted_value)

ulp = r_plus - r_minus

# Calculate ULP
ulp = abs(computed_value - wanted_value) / ulp

print(ulp)
