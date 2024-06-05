import pandas as pd

for config in ['32_G16', '32_Plonk', '64_G16', '64_Plonk']:
    # Path to read the data file
    file_path = '../benchmarks/raw/bench_ZKLP{}_BN254.txt'.format(config)

    # Reading the data from the .txt file
    df = pd.read_csv(file_path)

    # Calculating descriptive statistics for the entire dataset
    all_stats = df.describe()

    # Grouping the data by 'Resolution' and calculating descriptive statistics
    grouped_stats = df.groupby('Resolution').describe()

    # Path to save the results
    output_path = '../benchmarks/results/bench_ZKLP{}_BN254_results.txt'.format(config)

    # Saving the descriptive statistics to a .txt file
    with open(output_path, 'w') as file:
        file.write("Descriptive Statistics for the Entire Dataset:\n")
        file.write(all_stats.to_string())
        file.write("\n\nDescriptive Statistics Grouped by Resolution:\n")
        file.write(grouped_stats.to_string())
