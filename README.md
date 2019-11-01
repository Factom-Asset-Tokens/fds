# Factom Data Store

A robust protocol for storing data of nearly arbitrary size across many entries
on a Chain.

## Features
- Secure - ChainID uniquely identifies the exact data and metadata.
- Extensible - Applications may define any arbitrary metadata to attach to the
  data store.
- DOS proof - The data blocks may appear on chain in any order and may be
  interspersed with unrelated or garbage entries.
- Efficient - Clients only need to download the first modestly sized entry in
  the chain to discover the total data size, exact number of entries that they
will need to download, and can traverse a linked list of Data Block Entries to
avoid downloading any unrelated or garbage data.
- Arbitrary size - There is no theoretical limit to the size of data that may
  be stored. However most clients will likely enforce sane practical limits
before downloading a file.
- Censorship resistent - Commit all entries before revealing any to ensure that
  a data store cannot be censored.

## Specification

### First Entry

The First Entry establishes the file hash, size, application defined metadata,
and the first Data Block Entry. The chain ID is a hash of all of this data,
except for the Entry Hashes, which depend on the Chain ID and so appear in the
Content of the first entry, not the ExtIDs.

#### ExtIDs
| i   | Type      | Description                                                                    |
|-----|---------- |--------------------------------------------------------------------------------|
| 0   | string    | "data-store",  A human readable marker that defines this data protocol.        |
| 1   | varInt\_F | Version Number of the "data-store" protocol. Currently 0 (1 byte)              |
| 2   | varInt\_F | Total data size. Must be greater than 0.                                       |
| 3   | varInt\_F | Total number of Data Block Entries. Must be greater than 0.                    |
| 4   | Bytes32   | The sha256d hash of the data.                                                  |
| ... | (any)     | Any number of additional ExtIDs may be defined by the application as metadata. |

#### Content

The Content is the Hash of the first Data Block Entry.

### Data Block Entry

The data is split into a linked list of Data Block Entries, the first of which
is established in the First Entry of the Chain. The amount of data to include
in each entry is up to the application, but most use-cases will have no reason
not to fill the Entry to capacity.

#### ExtIDs
| i   | Type      | Description                                                                    |
|-----|---------- |--------------------------------------------------------------------------------|
| 0   | Bytes32   | The Entry Hash of the next Data Block Entry, if it exists.                     |
| ... | (any)     | Any number of additional ExtIDs may be defined by the application as metadata. |

#### Content

A block of the data.

### Writing a data store

1. Compute the Chain ID
    - Compute the total data size. `len(data)`
    - Compute the number of Data Block Entries. `len(data)/(10240 - 32 - 2)`
    - Compute the hash of the data. `sha256d(data)`
    - Append any application metadata.
    - Construct the ExtIDs of the First Entry.
    - Compute the ChainID
      `sha256(sha256(ExtID[0])|sha256(ExtID[1])|...|sha256(ExtID[n]))`

2. Build the Data Block Entries
    - Construct `len(data)/32` (`+1` if `len(data)%32 > 0`) Entries.
    - Sequentially fill the Content of the Entries with the data, and populate
      the ChainID.
    - Construct the linked list by traversing the Entries in reverse order to
      set up the `ExtID[0]` and compute the Entry Hashes.
    -  Set the Content of the first entry to the first Data Block Entry Hash in
       the linked list.

3. Publish the data store
    - Commit the first entry, and then commit all data block entries.
    - Wait for ACK for all Commits.
    - Reveal all entries.

### Reading a data store

1. Validate the data store chain structure and metadata.
    - Using a known Data Store Chain ID, download the First Entry.
    - Validate the protocol and version.
    - Ensure that the file is not too big or does not span too many Data Block
      Entries for the client.
    - If known, validate the declared data hash and size.
    - Apply any application specific validation rules for the metadata.

2. Reconstruct the data.
    - Starting from the first Data Block Entry from the First Entry Content,
      traverse the linked list, and concatenate the Content to reconstruct the
data.
    - Continue traversal until the declared data size or number of Data Block
      Entries reached, or until there is no subsequent Data Block Entry
declared.

3. Validate the data.
    - Validate the declared file size and number of Data Block Entries.
    - Compute and validate the declared file hash.
